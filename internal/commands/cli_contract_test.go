package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"gopkg.in/yaml.v3"
)

type cliOutput struct {
	stdout []byte
	stderr []byte
}

func invokeCLI(t *testing.T, args ...string) (cliOutput, error) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		t.Fatal(err)
	}
	type readResult struct {
		data []byte
		err  error
	}
	readPipe := func(reader *os.File) <-chan readResult {
		ch := make(chan readResult, 1)
		go func() {
			data, err := io.ReadAll(reader)
			_ = reader.Close()
			ch <- readResult{data: data, err: err}
		}()
		return ch
	}
	stdoutDone := readPipe(stdoutReader)
	stderrDone := readPipe(stderrReader)
	previousStdout, previousStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	runErr := Run(args)
	os.Stdout, os.Stderr = previousStdout, previousStderr
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	stdoutResult, stderrResult := <-stdoutDone, <-stderrDone
	if stdoutResult.err != nil {
		t.Fatalf("read CLI stdout: %v", stdoutResult.err)
	}
	if stderrResult.err != nil {
		t.Fatalf("read CLI stderr: %v", stderrResult.err)
	}
	return cliOutput{stdout: stdoutResult.data, stderr: stderrResult.data}, runErr
}

func changeWorkingDirectory(t *testing.T, dir string) func() {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to remain absent, stat error: %v", path, err)
	}
}

type cliFixture struct {
	t      *testing.T
	base   string
	root   string
	change string
	git    string
	source map[string]string
	remote map[string]string
}

func newCLIFixture(t *testing.T) *cliFixture {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("Git unavailable: %v", err)
	}
	base := t.TempDir()
	f := &cliFixture{
		t: t, base: base, root: filepath.Join(base, "agent"),
		change: filepath.Join(base, "change"), git: gitPath,
		source: map[string]string{}, remote: map[string]string{},
	}
	t.Setenv("AIW_GIT", gitPath)
	t.Setenv("AIW_HOME", filepath.Join(base, "aiw-home"))
	if out, err := invokeCLI(t, "init", f.root, "--skip-skills"); err != nil {
		t.Fatalf("initialize CLI fixture: %v\n%s", err, out.stderr)
	}
	f.configureGit(f.root)
	f.remote["agent"] = filepath.Join(base, "agent-remote.git")
	f.gitAt("", "init", "--bare", f.remote["agent"])
	f.gitAt(f.root, "remote", "add", "origin", f.remote["agent"])

	for _, name := range []string{"web", "api"} {
		f.source[name] = filepath.Join(base, name)
		f.remote[name] = filepath.Join(base, name+"-remote.git")
		f.gitAt("", "init", "-b", "main", f.source[name])
		f.configureGit(f.source[name])
		if err := os.WriteFile(filepath.Join(f.source[name], "README.md"), []byte(name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		f.gitAt(f.source[name], "add", "README.md")
		f.gitAt(f.source[name], "commit", "-m", "base")
		f.gitAt("", "init", "--bare", f.remote[name])
		f.gitAt(f.source[name], "remote", "add", "origin", f.remote[name])
		f.gitAt(f.source[name], "push", "-u", "origin", "main")
	}
	config := strings.Builder{}
	config.WriteString("version: 1\nname: integration\nbase: main\nrepos:\n")
	names := []string{"web", "api"}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&config, "  %s:\n    remote: %q\n    base: main\n", name, f.remote[name])
	}
	if err := os.WriteFile(filepath.Join(f.root, "aiw.yaml"), []byte(config.String()), 0644); err != nil {
		t.Fatal(err)
	}
	f.gitAt(f.root, "add", "-A")
	f.gitAt(f.root, "commit", "-m", "setup")
	f.gitAt(f.root, "push", "-u", "origin", "main")
	return f
}

func (f *cliFixture) gitAt(dir string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command(f.git, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v in %s: %v\n%s", args, dir, err, output)
	}
	return strings.TrimSpace(string(output))
}

func (f *cliFixture) configureGit(dir string) {
	f.t.Helper()
	f.gitAt(dir, "config", "user.name", "AIW Test")
	f.gitAt(dir, "config", "user.email", "aiw-test@example.com")
}

func (f *cliFixture) checkout() {
	f.t.Helper()
	f.gitAt(f.root, "worktree", "add", "-b", "feat/cli-contract", f.change, "main")
}

func TestCLIHelpAndCompletionBypassBusinessContext(t *testing.T) {
	workdir := t.TempDir()
	home := filepath.Join(workdir, "must-not-be-created")
	badGit := filepath.Join(workdir, "missing-git")
	t.Setenv("AIW_HOME", home)
	t.Setenv("AIW_GIT", badGit)
	t.Setenv("AIW_LIFECYCLE_ACTIVE", "1")
	restore := changeWorkingDirectory(t, workdir)
	defer restore()

	for _, args := range [][]string{
		{"--help"},
		{"help"},
		{"help", "repo", "list"},
		{"init", "--help"},
		{"version", "--help"},
		{"agents"},
		{"agents", "sync", "--help"},
		{"doctor", "--help"},
		{"repo"},
		{"repo", "--help"},
		{"repo", "list", "--help"},
		{"repo", "register", "--help"},
		{"change"},
		{"change", "--help"},
		{"change", "select", "--help"},
		{"change", "materialize", "--help"},
		{"change", "status", "--help"},
		{"change", "close", "--help"},
		{"completion", "--help"},
	} {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			out, err := invokeCLI(t, args...)
			if err != nil {
				t.Fatalf("Run(%q) failed without project context: %v\n%s", args, err, out.stderr)
			}
			if len(out.stderr) != 0 {
				t.Fatalf("Run(%q) wrote help output to stderr: %s", args, out.stderr)
			}
			if len(bytes.TrimSpace(out.stdout)) == 0 && len(bytes.TrimSpace(out.stderr)) == 0 {
				t.Fatalf("Run(%q) produced no help output", args)
			}
			assertPathAbsent(t, home)
		})
	}
	versionOutput, err := invokeCLI(t, "version")
	if err != nil || len(bytes.TrimSpace(versionOutput.stdout)) == 0 {
		t.Fatalf("version command failed without project context: %v\n%s", err, versionOutput.stderr)
	}
	assertPathAbsent(t, home)

	for _, shell := range []string{"bash", "zsh"} {
		shell := shell
		t.Run("completion_"+shell, func(t *testing.T) {
			out, err := invokeCLI(t, "completion", shell)
			if err != nil {
				t.Fatalf("generate %s completion: %v\n%s", shell, err, out.stderr)
			}
			if len(bytes.TrimSpace(out.stdout)) == 0 {
				t.Fatalf("generated %s completion is empty", shell)
			}
			binary, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s is unavailable for syntax validation", shell)
			}
			cmd := exec.Command(binary, "-n")
			cmd.Stdin = bytes.NewReader(out.stdout)
			if syntaxOutput, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s completion syntax check: %v\n%s", shell, err, syntaxOutput)
			}
			assertPathAbsent(t, home)
		})
	}

	versionCompletion, err := invokeCLI(t, "__complete", "version")
	if err != nil || !strings.Contains(string(versionCompletion.stdout), "version") {
		t.Fatalf("version command is missing from Cobra completion: %v\n%s", err, versionCompletion.stdout)
	}
	closeCompletion, err := invokeCLI(t, "__complete", "change", "close", "--")
	if err != nil {
		t.Fatalf("complete change close flags: %v\n%s", err, closeCompletion.stderr)
	}
	for _, flag := range []string{"--allow-unpushed", "--discard-changes"} {
		if !strings.Contains(string(closeCompletion.stdout), flag) {
			t.Errorf("close flag %s is missing from Cobra completion:\n%s", flag, closeCompletion.stdout)
		}
	}
	assertPathAbsent(t, home)
}

func TestCLIRootHelpForEmptyArgsIgnoresProcessArgs(t *testing.T) {
	workdir := t.TempDir()
	home := filepath.Join(workdir, "must-not-be-created")
	t.Setenv("AIW_HOME", home)
	t.Setenv("AIW_GIT", filepath.Join(workdir, "missing-git"))
	t.Setenv("AIW_LIFECYCLE_ACTIVE", "1")
	restore := changeWorkingDirectory(t, workdir)
	defer restore()

	previousArgs := os.Args
	os.Args = []string{"aiw", "version"}
	defer func() { os.Args = previousArgs }()

	cases := []struct {
		name string
		args []string
	}{
		{name: "nil"},
		{name: "empty", args: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := invokeCLI(t, tc.args...)
			if err != nil {
				t.Fatalf("empty Run(%s) should show root help: %v\n%s", tc.name, err, out.stderr)
			}
			if !strings.Contains(string(out.stdout), "Usage:") {
				t.Fatalf("empty Run(%s) did not show root help; process args may have leaked: %s", tc.name, out.stdout)
			}
			if len(out.stderr) != 0 {
				t.Fatalf("empty Run(%s) wrote help output to stderr: %s", tc.name, out.stderr)
			}
			assertPathAbsent(t, home)
		})
	}
}

func TestCLIReturnsErrorsWithoutPrinting(t *testing.T) {
	out, err := invokeCLI(t, "not-a-command")
	if err == nil {
		t.Fatal("unknown command unexpectedly succeeded")
	}
	if len(out.stdout) != 0 || len(out.stderr) != 0 {
		t.Fatalf("Run printed an error that main should own; stdout=%q stderr=%q", out.stdout, out.stderr)
	}
}

func TestCLIVersionOutput(t *testing.T) {
	previousVersion := aiw.BuildVersion
	aiw.BuildVersion = "v1.2.3"
	defer func() { aiw.BuildVersion = previousVersion }()
	out, err := invokeCLI(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if string(out.stdout) != "v1.2.3\n" {
		t.Fatalf("unexpected version output: %q", out.stdout)
	}
}

func TestCLIRejectsMalformedInputBeforeBusinessContext(t *testing.T) {
	workdir := t.TempDir()
	home := filepath.Join(workdir, "must-not-be-created")
	badGit := filepath.Join(workdir, "missing-git")
	t.Setenv("AIW_HOME", home)
	t.Setenv("AIW_GIT", badGit)
	// This also proves that leaf Args and flag checks run before Change's lifecycle guard.
	t.Setenv("AIW_LIFECYCLE_ACTIVE", "1")
	restore := changeWorkingDirectory(t, workdir)
	defer restore()

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "unknown-root-command", args: []string{"not-a-command"}, wantErr: "unknown command"},
		{name: "unknown-repo-command", args: []string{"repo", "not-a-command"}, wantErr: "unknown command"},
		{name: "unknown-change-command", args: []string{"change", "not-a-command"}, wantErr: "unknown command"},
		{name: "unknown-init-flag", args: []string{"init", "--not-a-flag"}, wantErr: "unknown flag"},
		{name: "unknown-repo-flag", args: []string{"repo", "list", "--not-a-flag"}, wantErr: "unknown flag"},
		{name: "unknown-change-flag", args: []string{"change", "close", "--not-a-flag"}, wantErr: "unknown flag"},
		{name: "missing-init-argument", args: []string{"init"}, wantErr: "accepts 1 arg"},
		{name: "empty-init-path", args: []string{"init", ""}, wantErr: "agent-repo-path must not be empty"},
		{name: "extra-init-argument", args: []string{"init", "one", "two"}, wantErr: "accepts 1 arg"},
		{name: "missing-register-name", args: []string{"repo", "register", "--source", filepath.Join(workdir, "source")}, wantErr: "accepts 1 arg"},
		{name: "missing-register-source", args: []string{"repo", "register", "web"}, wantErr: "--source is required"},
		{name: "empty-register-source", args: []string{"repo", "register", "web", "--source", ""}, wantErr: "--source is required and must not be empty"},
		{name: "extra-list-argument", args: []string{"repo", "list", "extra"}, wantErr: "unknown command"},
		{name: "missing-selection", args: []string{"change", "select"}, wantErr: "--repo is required"},
		{name: "empty-selection", args: []string{"change", "select", "--repo", ""}, wantErr: "--repo must not be empty"},
		{name: "missing-cleanup-name", args: []string{"change", "cleanup"}, wantErr: "accepts 1 arg"},
		{name: "extra-close-argument", args: []string{"change", "close", "extra"}, wantErr: "unknown command"},
		{name: "extra-path-argument", args: []string{"change", "path", "branch", "extra"}, wantErr: "accepts 1 arg"},
		{name: "extra-version-argument", args: []string{"version", "extra"}, wantErr: "unknown command"},
		{name: "extra-doctor-argument", args: []string{"doctor", "extra"}, wantErr: "unknown command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := invokeCLI(t, tc.args...)
			if err == nil {
				t.Fatalf("Run(%q) unexpectedly succeeded", tc.args)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Run(%q) returned %q; want argument error containing %q", tc.args, err, tc.wantErr)
			}
			if strings.Contains(err.Error(), badGit) || strings.Contains(err.Error(), home) {
				t.Fatalf("Run(%q) reached Git or AIW_HOME before rejecting its arguments: %v", tc.args, err)
			}
			if strings.Contains(err.Error(), "aiw.yaml not found") {
				t.Fatalf("Run(%q) searched for project context before rejecting its arguments: %v", tc.args, err)
			}
			if strings.Contains(err.Error(), "inside a repository setup or cleanup script") {
				t.Fatalf("Run(%q) reached the lifecycle guard before validating its arguments: %v", tc.args, err)
			}
			assertPathAbsent(t, home)
		})
	}
}

func TestCLILifecycleGuardRejectsValidChangeQueries(t *testing.T) {
	f := newCLIFixture(t)
	f.checkout()
	t.Setenv("AIW_LIFECYCLE_ACTIVE", "1")
	restore := changeWorkingDirectory(t, f.change)
	defer restore()

	for _, args := range [][]string{
		{"change", "status"},
		{"change", "list"},
		{"change", "path", "feat/cli-contract"},
	} {
		out, err := invokeCLI(t, args...)
		if err == nil || !strings.Contains(err.Error(), "inside a repository setup or cleanup script") {
			t.Errorf("Run(%q) should return the lifecycle recursion guard, got %v\n%s", args, err, out.stderr)
		}
		if len(out.stdout) != 0 || len(out.stderr) != 0 {
			t.Errorf("Run(%q) printed before returning the lifecycle guard: stdout=%q stderr=%q", args, out.stdout, out.stderr)
		}
	}
}

func TestCLIRunDoesNotLeakFlagState(t *testing.T) {
	f := newCLIFixture(t)
	restore := changeWorkingDirectory(t, f.root)
	defer restore()

	jsonOutput, err := invokeCLI(t, "repo", "list", "--json")
	if err != nil {
		t.Fatalf("first Run failed: %v\n%s", err, jsonOutput.stderr)
	}
	var rows []aiw.RepoListRow
	if err := json.Unmarshal(jsonOutput.stdout, &rows); err != nil || len(rows) != 2 {
		t.Fatalf("first Run did not emit repo JSON: %v\n%s", err, jsonOutput.stdout)
	}

	plainOutput, err := invokeCLI(t, "repo", "list")
	if err != nil {
		t.Fatalf("second Run failed: %v\n%s", err, plainOutput.stderr)
	}
	if bytes.HasPrefix(bytes.TrimSpace(plainOutput.stdout), []byte("[")) || !strings.Contains(string(plainOutput.stdout), "web") {
		t.Fatalf("--json leaked into the second root command: %s", plainOutput.stdout)
	}
}

func TestPublicRunRepoAndChangeWorkflow(t *testing.T) {
	f := newCLIFixture(t)
	t.Setenv("AIW_LIFECYCLE_ACTIVE", "")
	restore := changeWorkingDirectory(t, f.root)
	for _, args := range [][]string{
		{"repo", "register", "--source", f.source["web"], "web"},
		{"repo", "register", "api", "--source", f.source["api"]},
	} {
		out, err := invokeCLI(t, args...)
		if err != nil {
			t.Fatalf("Run(%q) failed: %v\n%s", args, err, out.stderr)
		}
	}
	listOutput, err := invokeCLI(t, "repo", "list", "--json")
	if err != nil {
		t.Fatalf("repo list --json failed: %v\n%s", err, listOutput.stderr)
	}
	var listed []aiw.RepoListRow
	if err := json.Unmarshal(listOutput.stdout, &listed); err != nil {
		t.Fatalf("repo list --json was not valid JSON: %v\n%s", err, listOutput.stdout)
	}
	if len(listed) != 2 {
		t.Fatalf("expected two configured repos, got %+v", listed)
	}
	for _, row := range listed {
		if !row.Registered || row.Source == "" || row.Store == "" {
			t.Errorf("repo %s was not registered through Run: %+v", row.Name, row)
		}
	}
	restore()

	f.checkout()
	restore = changeWorkingDirectory(t, f.change)
	if out, err := invokeCLI(t, "change", "select", "--repo", "web,api"); err == nil {
		t.Fatalf("--repo unexpectedly split a comma-separated value: %s", out.stdout)
	}
	selectionPath := filepath.Join(f.change, ".aiw", "change.yaml")
	if _, err := os.Stat(selectionPath); !os.IsNotExist(err) {
		t.Fatalf("invalid comma-separated repo flag wrote a selection: %v", err)
	}
	if out, err := invokeCLI(t, "change", "select", "--repo", "web", "--repo", "api"); err != nil {
		t.Fatalf("repeated --repo flags failed: %v\n%s", err, out.stderr)
	}
	selectionData, err := os.ReadFile(selectionPath)
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		Version int      `yaml:"version"`
		Repos   []string `yaml:"repos"`
	}
	if err := yaml.Unmarshal(selectionData, &selected); err != nil {
		t.Fatal(err)
	}
	sort.Strings(selected.Repos)
	if selected.Version != 1 || len(selected.Repos) != 2 || selected.Repos[0] != "api" || selected.Repos[1] != "web" {
		t.Fatalf("repeated --repo flags selected the wrong repositories: %+v", selected)
	}
	if out, err := invokeCLI(t, "change", "materialize"); err != nil {
		t.Fatalf("change materialize failed: %v\n%s", err, out.stderr)
	}
	statusOutput, err := invokeCLI(t, "change", "status", "--json")
	if err != nil {
		t.Fatalf("change status --json failed: %v\n%s", err, statusOutput.stderr)
	}
	var statusRows []aiw.StatusRow
	if err := json.Unmarshal(statusOutput.stdout, &statusRows); err != nil {
		t.Fatalf("change status --json was not valid JSON: %v\n%s", err, statusOutput.stdout)
	}
	if len(statusRows) != 3 || statusRows[0].Repo != "agent-repo" {
		t.Fatalf("expected root and two materialized repositories in status, got %+v", statusRows)
	}
	if out, err := invokeCLI(t, "change", "close", "--allow-unpushed"); err != nil {
		t.Fatalf("change close failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.stdout, out.stderr)
	}
	if _, err := os.Stat(f.change); err != nil {
		t.Fatalf("change close removed the control-plane worktree: %v", err)
	}
	for _, name := range []string{"web", "api"} {
		if _, err := os.Lstat(filepath.Join(f.change, "repos", name)); !os.IsNotExist(err) {
			t.Errorf("change close left the %s link behind: %v", name, err)
		}
	}
	restore()
}
