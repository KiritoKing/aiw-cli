package aiw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type integrationWorkspaceFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func assertIntegrationWorkspace(t *testing.T, root, name string, repos ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, vscodeWorkspaceFile))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("workspace must replace all previous content, got %s", data)
	}
	var folders []integrationWorkspaceFolder
	if err := json.Unmarshal(fields["folders"], &folders); err != nil {
		t.Fatal(err)
	}
	want := []integrationWorkspaceFolder{{Name: name, Path: "."}}
	for _, repo := range repos {
		want = append(want, integrationWorkspaceFolder{Name: repo, Path: "repos/" + repo})
	}
	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("folders = %+v, want %+v", folders, want)
	}
}

func TestWorkspaceSyncCommandUsesCurrentConfigAndReplacesFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "agent")
	if err := Run([]string{"init", root, "--skip-skills"}); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegrationWorkspace(t, root, "agent")
	ignoreBefore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	cfg.VSCode.AutoSync = &disabled
	cfg.Name = "跨仓代码"
	cfg.Repos = map[string]RepoConfig{
		"web": {Remote: "git@example.com:team/web.git", Base: "main"},
		"api": {Remote: "git@example.com:team/api.git", Base: "main"},
	}
	if err := saveProject(root, cfg); err != nil {
		t.Fatal(err)
	}
	otherWorkspace := filepath.Join(root, "custom.code-workspace")
	if err := os.WriteFile(otherWorkspace, []byte("personal workspace\n"), 0644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "notes", "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
	aiwHome := filepath.Join(t.TempDir(), "unused-home")
	t.Setenv("AIW_HOME", aiwHome)
	output := captureStdout(t, func() error { return Run([]string{"workspace", "sync"}) })
	if !strings.Contains(string(output), filepath.Join(root, vscodeWorkspaceFile)) {
		t.Fatalf("output does not identify the generated workspace: %s", output)
	}
	assertIntegrationWorkspace(t, root, "跨仓代码", "api", "web")
	if _, err := os.Stat(aiwHome); !os.IsNotExist(err) {
		t.Fatalf("manual sync touched AIW_HOME: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "repos")); !os.IsNotExist(err) {
		t.Fatalf("workspace sync created business paths: %v", err)
	}
	if _, err := git(root, "check-ignore", "--quiet", vscodeWorkspaceFile); err != nil {
		t.Fatalf("generated workspace is not locally ignored: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, vscodeWorkspaceFile), []byte(`{"folders": [], "settings": {"editor.fontSize": 30}, "tasks": {}}`), 0644); err != nil {
		t.Fatal(err)
	}
	delete(cfg.Repos, "web")
	cfg.Repos["worker"] = RepoConfig{Remote: "git@example.com:team/worker.git", Base: "main"}
	if err := saveProject(root, cfg); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"workspace", "sync"}); err != nil {
		t.Fatal(err)
	}
	assertIntegrationWorkspace(t, root, "跨仓代码", "api", "worker")
	ignoreAfter, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || string(ignoreAfter) != string(ignoreBefore) {
		t.Fatalf("sync changed tracked ignore configuration: %v, %q", err, ignoreAfter)
	}
	otherData, err := os.ReadFile(otherWorkspace)
	if err != nil || string(otherData) != "personal workspace\n" {
		t.Fatalf("sync changed another workspace file: %v, %q", err, otherData)
	}
}

func TestWorkspaceSyncRejectsArgumentsBeforeDiscovery(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
	for _, args := range [][]string{
		{"workspace"},
		{"workspace", "unknown"},
		{"workspace", "sync", "extra"},
		{"workspace", "sync", "--unknown"},
	} {
		if err := Run(args); err == nil || !strings.Contains(err.Error(), "usage: aiw workspace sync") {
			t.Fatalf("%v should fail usage validation before project discovery, got %v", args, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid command wrote files: %v, %v", entries, err)
	}
}

func TestProjectVSCodeAutoSyncConfiguration(t *testing.T) {
	base := "version: 1\nname: example\nbase: main\nrepos: {}\n"
	for _, tc := range []struct {
		name string
		yaml string
		want *bool
	}{
		{name: "default"},
		{name: "enabled", yaml: "vscode:\n  autoSync: true\n", want: newWorkspaceBool(true)},
		{name: "disabled", yaml: "vscode:\n  autoSync: false\n", want: newWorkspaceBool(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseProject([]byte(base + tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.VSCode.AutoSync, tc.want) {
				t.Fatalf("autoSync = %v, want %v", cfg.VSCode.AutoSync, tc.want)
			}
			root := t.TempDir()
			if err := saveProject(root, cfg); err != nil {
				t.Fatal(err)
			}
			reloaded, err := loadProject(root)
			if err != nil || !reflect.DeepEqual(reloaded.VSCode.AutoSync, tc.want) {
				t.Fatalf("autoSync did not survive save/load: %+v, %v", reloaded.VSCode, err)
			}
		})
	}
	for _, invalid := range []string{"vscode: false\n", "vscode:\n  autoSync: nope\n", "vscode:\n  autoSync: []\n"} {
		if _, err := parseProject([]byte(base + invalid)); err == nil {
			t.Fatalf("invalid workspace setting accepted: %q", invalid)
		}
	}
}

func newWorkspaceBool(value bool) *bool { return &value }

func TestWorkspaceAutomaticMaterializeAndRepair(t *testing.T) {
	f := newFixture(t)
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	f.mustRun(Run([]string{"change", "materialize"}))
	assertIntegrationWorkspace(t, c.Root, "integration", "api", "web")
	if _, err := os.Lstat(c.linkPath("api")); !os.IsNotExist(err) {
		t.Fatalf("workspace generation materialized an unselected repo: %v", err)
	}
	if _, err := git(c.Root, "check-ignore", "--quiet", vscodeWorkspaceFile); err != nil {
		t.Fatalf("workspace is not ignored in Change worktree: %v", err)
	}
	if err := os.Remove(filepath.Join(c.Root, vscodeWorkspaceFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.linkPath("web")); err != nil {
		t.Fatal(err)
	}
	f.mustRun(Run([]string{"change", "repair"}))
	assertIntegrationWorkspace(t, c.Root, "integration", "api", "web")
	if _, err := os.Readlink(c.linkPath("web")); err != nil {
		t.Fatalf("repair did not restore the selected repo link: %v", err)
	}
	cfg := c.Config
	cfg.VSCode.AutoSync = newWorkspaceBool(false)
	f.mustRun(saveProject(c.Root, cfg))
	const retained = "retain manually maintained content\n"
	if err := os.WriteFile(filepath.Join(c.Root, vscodeWorkspaceFile), []byte(retained), 0644); err != nil {
		t.Fatal(err)
	}
	f.mustRun(Run([]string{"change", "materialize"}))
	f.mustRun(Run([]string{"change", "repair"}))
	data, err := os.ReadFile(filepath.Join(c.Root, vscodeWorkspaceFile))
	if err != nil || string(data) != retained {
		t.Fatalf("disabled auto-sync changed the workspace: %v, %q", err, data)
	}
	s, err := c.state()
	if err != nil || s.Phase != "ready" {
		t.Fatalf("workspace sync affected business readiness: %+v, %v", s, err)
	}
}

func TestWorkspaceAutoSyncFailureBeforeMaterializeCanRetry(t *testing.T) {
	f := newFixture(t)
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	outputPath := filepath.Join(c.Root, vscodeWorkspaceFile)
	if err := os.Mkdir(outputPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := c.materialize(false); err == nil || !strings.Contains(err.Error(), vscodeWorkspaceFile) {
		t.Fatalf("automatic sync failure was hidden: %v", err)
	}
	if _, err := os.Lstat(c.worktreePath("web")); !os.IsNotExist(err) {
		t.Fatalf("business worktree was created despite failed preflight: %v", err)
	}
	if err := os.Remove(outputPath); err != nil {
		t.Fatal(err)
	}
	f.mustRun(c.materialize(false))
	assertIntegrationWorkspace(t, c.Root, "integration", "api", "web")
	s, err := c.state()
	if err != nil || s.Phase != "ready" {
		t.Fatalf("retry did not finish materialization: %+v, %v", s, err)
	}
}
