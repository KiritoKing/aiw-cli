package aiw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func runCLI(cwd string, args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	err = RunWithIO(context.Background(), args, IO{
		Cwd: cwd, In: strings.NewReader(""), Out: &out, ErrOut: &errOut,
	})
	return out.String(), errOut.String(), err
}

func TestCobraHelpAndArgumentValidationOutsideProject(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "unused-home")
	t.Setenv("AIW_HOME", home)
	t.Setenv("AIW_GIT", filepath.Join(root, "missing-git"))
	for _, args := range [][]string{
		{"repo", "--help"}, {"change", "-h"}, {"change", "materialize", "--help"},
		{"agents", "sync", "-h"}, {"doctor", "--help"},
	} {
		out, errOut, err := runCLI(root, args...)
		if err != nil || !strings.Contains(out, "Usage:") || errOut != "" {
			t.Fatalf("help %v: stdout=%q stderr=%q err=%v", args, out, errOut, err)
		}
	}
	for _, args := range [][]string{
		{"workspace"}, {"repo"}, {"change"}, {"agents"}, {"completion"},
		{"repo", "register", "web"}, {"repo", "register", "web", "--source", ".", "extra"},
		{"change", "list", "extra"}, {"change", "materialize", "extra"}, {"change", "close", "--invalid"},
		{"change", "select"}, {"change", "select", "--no-input"}, {"doctor", "extra"},
	} {
		out, errOut, err := runCLI(root, args...)
		if err == nil || strings.Contains(err.Error(), "aiw.yaml") || out != "" || errOut != "" {
			t.Fatalf("invalid %v reached project discovery or emitted usage: stdout=%q stderr=%q err=%v", args, out, errOut, err)
		}
	}
	for _, flag := range []string{"-v", "--version", "version"} {
		out, errOut, err := runCLI(root, flag)
		if err != nil || errOut != "" || out != buildVersion()+"\n" {
			t.Fatalf("version %s: stdout=%q stderr=%q err=%v", flag, out, errOut, err)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("help or argument validation created AIW_HOME: %v", err)
	}
}

func TestCobraCwdAndDynamicCompletion(t *testing.T) {
	f := newFixture(t)
	const branch = "feat/integration"
	cfg, err := testEnv().loadProject(f.root)
	f.mustRun(err)
	web := cfg.Repos["web"]
	web.Metadata = map[string]any{"description": "Customer web"}
	cfg.Repos["web"] = web
	f.mustRun(saveProject(f.root, cfg))
	out, errOut, err := runCLI(f.base, "repo", "register", "--source", "../web", "web", "-C", "agent")
	if err != nil || errOut != "" || !strings.Contains(out, "registered web:") {
		t.Fatalf("relative --cwd and flag order: stdout=%q stderr=%q err=%v", out, errOut, err)
	}
	out, errOut, err = runCLI(f.base, "--cwd", "agent", "repo", "list", "--json")
	if err != nil || errOut != "" || !strings.Contains(out, f.source["web"]) {
		t.Fatalf("long --cwd: stdout=%q stderr=%q err=%v", out, errOut, err)
	}

	for _, sub := range [][]string{
		{"repo", "register"}, {"repo", "sync"}, {"change", "add-repo"},
		{"change", "remove-repo"}, {"change", "setup"}, {"change", "cleanup"},
		{"change", "select", "--repo"},
	} {
		args := append([]string{"__complete", "-C", "agent"}, sub...)
		args = append(args, "w")
		out, _, err := runCLI(f.base, args...)
		if err != nil || out != "web\tbase main · Customer web\n:4\n" {
			t.Fatalf("complete %v: %q, %v", sub, out, err)
		}
	}
	out, _, err = runCLI(f.root, "__complete", "change", "select", "--repo", "web", "--repo", "")
	if err != nil || strings.Contains(out, "web") || !strings.Contains(out, "api\tbase main") {
		t.Fatalf("repeatable repo flag completion: %q, %v", out, err)
	}
	project, err := testEnv().gitCommonDir(f.root)
	f.mustRun(err)
	f.mustRun(saveState(os.Getenv("AIW_HOME"), ChangeState{Branch: branch, Root: f.root, Project: project, Phase: "ready"}))
	t.Setenv("AIW_HOME", "../aiw-home")
	for _, verb := range []string{"path", "open"} {
		out, _, err = runCLI(f.base, "-C", "agent", "__complete", "change", verb, "feat/")
		if err != nil || out != branch+"\tready\n:4\n" {
			t.Fatalf("complete change %s: %q, %v", verb, out, err)
		}
	}
}

func TestChangeListAndDoctorJSON(t *testing.T) {
	f := newFixture(t)
	home := os.Getenv("AIW_HOME")
	f.mustRun(testEnv().registerRepo(f.root, home, "web", f.source["web"]))
	f.checkout()
	f.spec("web")
	_, _, err := runCLI(f.change, "change", "materialize")
	f.mustRun(err)
	project, err := testEnv().gitCommonDir(f.root)
	f.mustRun(err)
	f.mustRun(saveState(home, ChangeState{Project: project, Root: f.root, Branch: "a-available", Phase: "closed"}))
	f.mustRun(os.Rename(f.change, filepath.Join(f.base, "moved-change")))

	out, stderr, err := runCLI(f.root, "change", "list", "--json")
	var rows []ChangeListRow
	if err != nil || stderr != "" || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 2 ||
		rows[0].Branch != "a-available" || !rows[0].RootExists || rows[1].Branch != "feat/integration" || rows[1].RootExists {
		t.Fatalf("change list JSON: stdout=%q stderr=%q err=%v", out, stderr, err)
	}
	textOutput, _, err := runCLI(f.root, "change", "list")
	if err != nil || !strings.Contains(textOutput, "stale") || strings.ContainsRune(textOutput, '\x1b') {
		t.Fatalf("change list text: %q, %v", textOutput, err)
	}

	gitPath, err := exec.LookPath(gitBin())
	f.mustRun(err)
	t.Setenv("AIW_GIT", gitPath)
	t.Setenv("PATH", t.TempDir())
	out, stderr, err = runCLI(f.root, "doctor", "--json")
	report := parseDoctorReport(t, out, stderr)
	if err != nil || !report.OK || report.Root != f.root {
		t.Fatalf("doctor report: %+v, %v", report, err)
	}
	want := map[string]string{
		"aiw": "pass", "aiw-home": "pass", "git": "pass", "node": "info", "npx": "info",
		"aiw.yaml": "pass", "repos-ignore": "pass", "agents-block": "pass", "store/web": "pass",
		"store/api": "info", "orphan/feat/integration": "warn",
	}
	for _, check := range report.Checks {
		if status, ok := want[check.Name]; ok {
			if check.Status != status || check.Detail == "" {
				t.Fatalf("%s: got %+v, want %s", check.Name, check, status)
			}
			delete(want, check.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing doctor checks: %v", want)
	}

	f.mustRun(os.WriteFile(filepath.Join(f.root, "AGENTS.md"), []byte(agentStart+"\n"), 0644))
	f.mustRun(os.WriteFile(filepath.Join(f.root, ".gitignore"), nil, 0644))
	repoState, err := loadState(home, project, "feat/integration")
	f.mustRun(err)
	f.mustRun(os.Rename(repoState.Repos["web"].Store, repoState.Repos["web"].Store+".missing"))
	out, stderr, err = runCLI(f.root, "doctor", "--json")
	report = parseDoctorReport(t, out, stderr)
	if err == nil || report.OK {
		t.Fatalf("doctor should identify failing checks: %+v, %v", report, err)
	}
	failed := map[string]bool{}
	for _, check := range report.Checks {
		failed[check.Name] = check.Status == "fail"
	}
	for _, name := range []string{"agents-block", "repos-ignore", "store/web"} {
		if !failed[name] {
			t.Fatalf("doctor did not fail %s: %+v", name, report)
		}
	}
	t.Setenv("AIW_HOME", filepath.Join(f.root, "AGENTS.md", "impossible"))
	out, stderr, err = runCLI(f.base, "doctor", "--json")
	report = parseDoctorReport(t, out, stderr)
	if err == nil || report.OK || report.Root != "" || report.Checks[1].Status != "fail" {
		t.Fatalf("doctor should report an invalid AIW_HOME outside a project: %+v, %v", report, err)
	}
}

func parseDoctorReport(t *testing.T, out, stderr string) DoctorReport {
	t.Helper()
	var report DoctorReport
	if err := json.Unmarshal([]byte(out), &report); err != nil || stderr != "" {
		t.Fatalf("doctor output: stdout=%q stderr=%q parse error=%v", out, stderr, err)
	}
	return report
}

func TestStrictYAMLAndRemoteValidation(t *testing.T) {
	isolateGitConfig(t)
	project := "version: 1\nname: example\nbase: main\n"
	r := testEnv()
	for _, tc := range []struct{ body, message string }{
		{"reops: {}", "field reops"},
		{"repos:\n  web:\n    remote: https://example.test/web.git\n    base: main\n    setup: {comand: true}", "field comand"},
		{"repos:\n  web:\n    remote: https://opaque-token@example.test/web.git\n    base: main", "must not embed credentials"},
		{"repos: {}\n---\nextra: true", "single YAML document"},
	} {
		_, err := r.parseProject([]byte(project + tc.body + "\n"))
		if err == nil || !strings.Contains(err.Error(), tc.message) || strings.Contains(err.Error(), "opaque-token") {
			t.Fatalf("expected %q, got %v", tc.message, err)
		}
	}
	cfg, err := r.parseProject([]byte(project + "repos:\n  web:\n    remote: ssh://git@example.test/web.git\n    base: main\n    metadata: {custom: {free: 42}}\n"))
	if err != nil || !reflect.DeepEqual(cfg.Repos["web"].Metadata["custom"], map[string]any{"free": 42}) {
		t.Fatalf("SSH and metadata failed: %+v, %v", cfg, err)
	}
	root := t.TempDir()
	if err := atomicWrite(specPath(root), []byte("version: 1\nrepos: [web]\nrepoos: [api]\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSpec(root, cfg); err == nil || !strings.Contains(err.Error(), "change.yaml") || !strings.Contains(err.Error(), "field repoos") {
		t.Fatalf("unknown change field: %v", err)
	}
	for _, remote := range []string{"ssh://git@example.test:22/team/web.git/", "https://example.test:443/team/web///", "git@EXAMPLE.TEST:team/web.git/"} {
		if got := canonicalRemote(remote); got != "example.test/team/web" {
			t.Errorf("normalize %s: %q", remote, got)
		}
	}
	if got := canonicalRemote("ssh://git@example.test:2222/team/web.git"); got != "example.test:2222/team/web" {
		t.Fatalf("non-default port lost: %q", got)
	}
	registry := Registry{Repos: map[string]RegistryEntry{"example.test:22/team/web": {
		Remote: "ssh://git@example.test:22/team/web.git", Store: "/legacy/store",
	}}}
	if err := saveRegistry(root, registry); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadRegistry(root)
	if err != nil || loaded.Repos["example.test/team/web"].Store != "/legacy/store" {
		t.Fatalf("legacy registry: %+v, %v", loaded, err)
	}
}

func TestAtomicWritePreservesModeAndRejectsUnsafeTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	assertWrite := func(want os.FileMode) {
		t.Helper()
		if err := atomicWrite(path, []byte("updated")); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("mode should be %o: %v, %v", want, info, err)
		}
	}
	assertWrite(0644)
	if err := os.Chmod(path, 0750); err != nil {
		t.Fatal(err)
	}
	assertWrite(0750)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{dir, link} {
		if err := atomicWrite(target, []byte("unsafe")); err == nil {
			t.Fatalf("accepted non-regular target %s", target)
		}
	}
	if dest, err := os.Readlink(link); err != nil || dest != path {
		t.Fatalf("symbolic link changed: %q, %v", dest, err)
	}
}

func TestLockWaitCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.lock")
	first := flock.New(path)
	defer first.Close()
	if err := first.Lock(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var stderr bytes.Buffer
	env := newCommandEnv(ctx, IO{Out: io.Discard, ErrOut: &stderr})
	err := env.withLock(path, func() error {
		t.Error("held lock executed the action")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(stderr.String(), "waiting for lock "+path) {
		t.Fatalf("lock wait: stderr=%q err=%v", stderr.String(), err)
	}
	if err := first.Unlock(); err != nil {
		t.Fatal(err)
	}
	called := false
	env = newCommandEnv(context.Background(), IO{Out: io.Discard, ErrOut: &stderr})
	if err := env.withLock(path, func() error { called = true; return nil }); err != nil || !called {
		t.Fatalf("uncontended lock: called=%t err=%v", called, err)
	}
}

func TestGitRunnerDisablesPromptRedactsAndCancels(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "fake-git")
	if err := os.WriteFile(executable, []byte(`#!/bin/sh
if [ "$1" = hold ]; then exec /bin/sleep 60; fi
printf '%s' "$GIT_TERMINAL_PROMPT"
`), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIW_GIT", executable)
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	var stderr bytes.Buffer
	r := gitRunner{ctx: context.Background(), verbose: &stderr}
	out, err := r.git(dir, "remote", "https://user:password@example.test/repo?credential=value")
	if err != nil || out != "0" {
		t.Fatalf("prompt environment: %q, %v", out, err)
	}
	trace := stderr.String()
	if !strings.Contains(trace, "git [") || !strings.Contains(trace, "example.test/repo") ||
		strings.Contains(trace, "password") || strings.Contains(trace, "credential=value") {
		t.Fatalf("unexpected redacted trace: %q", trace)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	r.ctx = ctx
	if _, err := r.git(dir, "hold"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Git did not stop on context cancellation: %v", err)
	}
}
