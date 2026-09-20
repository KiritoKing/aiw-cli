package aiw

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	aiwskills "github.com/KiritoKing/aiw-cli/skills"
)

func TestAgentBlockCreationAndReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	userPrefix := []byte("# Team rules\r\n\r\nKeep this rule.\r\n")
	userSuffix := []byte("\r\n## More rules\r\nKeep that rule.\r\n")
	initial := append(append(append([]byte(nil), userPrefix...), []byte(agentStart+"\r\nold AIW text\r\n"+agentEnd)...), userSuffix...)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	if err := syncAgentInstructions(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first, userPrefix) || !bytes.HasSuffix(first, userSuffix) || bytes.Contains(first, []byte("old AIW text")) || !bytes.Contains(first, []byte("independent Git histories")) {
		t.Fatalf("managed block replacement changed user text or missed AIW rules: %q", first)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file mode changed: %v, %v", info, err)
	}
	if err := syncAgentInstructions(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("sync is not idempotent: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := syncAgentInstructions(filepath.Dir(missing)); err != nil {
		t.Fatal(err)
	}
	created, err := os.ReadFile(missing)
	if err != nil || !bytes.HasPrefix(created, []byte(agentStart)) {
		t.Fatalf("missing file was not created: %v", err)
	}
	appendRoot := t.TempDir()
	userText := []byte("# User\nDo not change this.")
	if err := os.WriteFile(filepath.Join(appendRoot, "AGENTS.md"), userText, 0644); err != nil {
		t.Fatal(err)
	}
	if err := syncAgentInstructions(appendRoot); err != nil {
		t.Fatal(err)
	}
	appended, err := os.ReadFile(filepath.Join(appendRoot, "AGENTS.md"))
	if err != nil || !bytes.HasPrefix(appended, userText) || bytes.Count(appended, []byte(agentStart)) != 1 {
		t.Fatalf("failed to append managed block: %v", err)
	}
}

func TestAgentBlockRejectsMalformedAndSymlink(t *testing.T) {
	for _, content := range []string{
		agentStart + "\nmissing end\n",
		agentEnd + "\nmissing start\n",
		agentEnd + "\n" + agentStart + "\n",
		agentStart + "\n" + agentStart + "\n" + agentEnd + "\n",
		"prefix " + agentStart + "\n" + agentEnd + "\n",
	} {
		t.Run(strings.ReplaceAll(content, "\n", "_"), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "AGENTS.md")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			if err := syncAgentInstructions(root); err == nil {
				t.Fatal("expected malformed marker error")
			}
			got, _ := os.ReadFile(path)
			if string(got) != content {
				t.Fatal("malformed AGENTS.md changed")
			}
		})
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := syncAgentInstructions(root); err == nil {
		t.Fatal("expected symlink rejection")
	}
	got, _ := os.ReadFile(outside)
	if string(got) != "outside" {
		t.Fatal("symlink target changed")
	}
}

func TestInitSkipSkills(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git unavailable")
	}
	t.Setenv("AIW_GIT", gitPath)
	t.Setenv("PATH", t.TempDir())
	root := filepath.Join(t.TempDir(), "agent")
	if err := runInit([]string{root, "--skip-skills"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	toolchain, err := os.ReadFile(toolchainPath(root))
	if err != nil || !bytes.Contains(toolchain, []byte(`AIW_VERSION="dev"`)) || !bytes.Contains(toolchain, []byte("AIW_SHA256_LINUX_AMD64")) {
		t.Fatalf("init did not write an AIW toolchain lock: %v\n%s", err, toolchain)
	}
	bootstrap := bootstrapPath(root)
	info, err := os.Stat(bootstrap)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("init did not write an executable bootstrap script: %v, %v", info, err)
	}
	script, err := os.ReadFile(bootstrap)
	if err != nil || !bytes.Contains(script, []byte("checksum mismatch")) || !bytes.Contains(script, []byte("installed AIW version")) {
		t.Fatalf("bootstrap script is incomplete: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("--skip-skills installed skills: %v", err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	if err := Run([]string{"agents", "sync"}); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedBootstrapRejectsIncompleteLockBeforeDownload(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git unavailable")
	}
	previousVersion := BuildVersion
	BuildVersion = "v1.2.3"
	defer func() { BuildVersion = previousVersion }()
	t.Setenv("AIW_GIT", gitPath)
	root := filepath.Join(t.TempDir(), "agent")
	if err := runInit([]string{root, "--skip-skills"}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bootstrapPath(root))
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "AIW_ARTIFACT_BASE_URL is required") {
		t.Fatalf("bootstrap should reject an incomplete lock before download: %v\n%s", err, output)
	}
}

func TestVersionOutput(t *testing.T) {
	previousVersion := BuildVersion
	BuildVersion = "v1.2.3"
	defer func() { BuildVersion = previousVersion }()
	output := captureStdout(t, func() error { return Run([]string{"version"}) })
	if string(output) != "v1.2.3\n" {
		t.Fatalf("unexpected version output: %q", output)
	}
}

func TestBundledSkillsFallbackAndConflict(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	if err := installSkills(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range aiwSkillNames {
		want, err := aiwskills.Files.ReadFile(name + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, ".agents", "skills", name, "SKILL.md"))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("bundled skill %s mismatch: %v", name, err)
		}
	}
	if err := installSkills(root); err != nil {
		t.Fatalf("identical install should be idempotent: %v", err)
	}
	conflict := filepath.Join(root, ".agents", "skills", "aiw-init", "SKILL.md")
	if err := os.WriteFile(conflict, []byte("user edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installSkills(root); err == nil {
		t.Fatal("expected user edit conflict")
	}
	got, _ := os.ReadFile(conflict)
	if string(got) != "user edited" {
		t.Fatal("user skill edit overwritten")
	}
}

func TestNpxSkillInstallerArguments(t *testing.T) {
	root, bin := t.TempDir(), t.TempDir()
	for name, script := range map[string]string{
		"node": "#!/bin/sh\nexit 0\n",
		"npx":  "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$AIW_NPX_ARGS\"\n/bin/mkdir -p .agents/skills/aiw-init .agents/skills/aiw-reference\nprintf '%s\\n' 'AIW-GO-WORKTREE-V1' > .agents/skills/aiw-init/SKILL.md\nprintf '%s\\n' 'AIW-GO-WORKTREE-V1' > .agents/skills/aiw-reference/SKILL.md\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("PATH", bin)
	t.Setenv("AIW_NPX_ARGS", argsFile)
	if err := installSkills(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "--yes\nskills\nadd\nKiritoKing/aiw-cli\n--skill\naiw-init\n--skill\naiw-reference\n--agent\nuniversal\n--copy\n--yes\n"
	if string(got) != want {
		t.Fatalf("unexpected npx arguments:\n%s", got)
	}
}

func TestNpxFailureDoesNotUseBundledFallback(t *testing.T) {
	root, bin := t.TempDir(), t.TempDir()
	for name, script := range map[string]string{
		"node": "#!/bin/sh\nexit 0\n",
		"npx":  "#!/bin/sh\nexit 17\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	if err := installSkills(root); err == nil || !strings.Contains(err.Error(), "npx skills add failed") {
		t.Fatalf("expected npx failure, got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("unexpected bundled fallback: %v", err)
	}
}

func TestNpxRejectsOutdatedSkills(t *testing.T) {
	root, bin := t.TempDir(), t.TempDir()
	for name, script := range map[string]string{
		"node": "#!/bin/sh\nexit 0\n",
		"npx": "#!/bin/sh\n/bin/mkdir -p .agents/skills/aiw-init .agents/skills/aiw-reference\n" +
			"printf '%s\\n' 'old workflow' > .agents/skills/aiw-init/SKILL.md\n" +
			"printf '%s\\n' 'old workflow' > .agents/skills/aiw-reference/SKILL.md\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	if err := installSkills(root); err == nil || !strings.Contains(err.Error(), "outdated") {
		t.Fatalf("expected outdated skill error, got %v", err)
	}
}

func captureStdout(t *testing.T, run func() error) []byte {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	runErr := run()
	os.Stdout = previous
	writer.Close()
	output, readErr := io.ReadAll(reader)
	reader.Close()
	if runErr != nil {
		t.Fatal(runErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	return output
}

func TestRepoListAndChangeStatusJSON(t *testing.T) {
	f := newFixture(t)
	f.mustRun(scanRepos(f.root, os.Getenv("AIW_HOME"), f.base))
	cfg, err := loadProject(f.root)
	if err != nil {
		t.Fatal(err)
	}
	api := cfg.Repos["api"]
	api.Metadata = map[string]any{"description": "API service", "tags": []string{"backend"}, "runtime": map[string]any{"go": "1.22"}}
	cfg.Repos["api"] = api
	if err := saveProject(f.root, cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadProject(f.root)
	if err != nil || reloaded.Repos["api"].Metadata["description"] != "API service" {
		t.Fatalf("metadata did not survive config round trip: %v", err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(f.root); err != nil {
		t.Fatal(err)
	}
	listOutput := captureStdout(t, func() error { return runRepo([]string{"list", "--json"}) })
	if err := os.Chdir(previous); err != nil {
		t.Fatal(err)
	}
	var listed []RepoListRow
	if err := json.Unmarshal(listOutput, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Name != "api" || !listed[0].Registered || listed[0].Source != f.source["api"] || listed[0].Store == "" || listed[0].Metadata["description"] != "API service" {
		t.Fatalf("unexpected repo list: %+v", listed)
	}
	f.checkout()
	c := f.context()
	f.spec("web")
	f.mustRun(c.materialize(false))
	state, err := c.state()
	if err != nil {
		t.Fatal(err)
	}
	statusOutput := captureStdout(t, func() error { return c.status(true) })
	var rows []StatusRow
	if err := json.Unmarshal(statusOutput, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Repo != "agent-repo" || rows[0].BaseSHA != "" || rows[1].BaseSHA != state.Repos["web"].Base || rows[1].State.Branch != c.Branch {
		t.Fatalf("unexpected status rows: %+v", rows)
	}
}
