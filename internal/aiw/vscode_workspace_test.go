package aiw

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newVSCodeTestRepo(t *testing.T) string {
	t.Helper()
	isolateGitConfig(t)
	if _, err := testEnv().git("", "--version"); err != nil {
		t.Skipf("Git unavailable: %v", err)
	}
	root := filepath.Join(t.TempDir(), "agent")
	if _, err := testEnv().git("", "init", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	return root
}

func vscodeTestExcludePath(t *testing.T, root string) string {
	t.Helper()
	path, err := testEnv().git(root, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return path
}

func readVSCodeWorkspace(t *testing.T, root string) ([]byte, vscodeWorkspace) {
	t.Helper()
	path := filepath.Join(root, vscodeWorkspaceFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("invalid workspace JSON: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("workspace should contain only folders, got keys %v", fields)
	}
	var workspace vscodeWorkspace
	if err := json.Unmarshal(data, &workspace); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}
	return data, workspace
}

func TestSyncVSCodeWorkspaceUsesAllConfiguredReposInStableOrder(t *testing.T) {
	root := newVSCodeTestRepo(t)
	cfg := ProjectConfig{
		Name: "agent-root",
		Repos: map[string]RepoConfig{
			"zeta":  {},
			"alpha": {},
		},
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err != nil {
		t.Fatal(err)
	}
	data, workspace := readVSCodeWorkspace(t, root)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("workspace must end with a newline")
	}
	want := []vscodeWorkspaceFolder{
		{Name: "agent-root", Path: "."},
		{Name: "alpha", Path: filepath.Join("repos", "alpha")},
		{Name: "zeta", Path: filepath.Join("repos", "zeta")},
	}
	if len(workspace.Folders) != len(want) {
		t.Fatalf("got %d folders, want %d: %#v", len(workspace.Folders), len(want), workspace.Folders)
	}
	for i := range want {
		if workspace.Folders[i] != want[i] {
			t.Fatalf("folder %d = %#v, want %#v", i, workspace.Folders[i], want[i])
		}
	}
	if strings.Contains(string(data), root) {
		t.Fatalf("workspace contains an absolute root path: %s", data)
	}
	if _, err := os.Lstat(filepath.Join(root, "repos")); !os.IsNotExist(err) {
		t.Fatalf("workspace sync must not create business directories: %v", err)
	}
}

func TestSyncVSCodeWorkspaceEmptyReposAndReplacesWholeFile(t *testing.T) {
	root := newVSCodeTestRepo(t)
	path := filepath.Join(root, vscodeWorkspaceFile)
	old := []byte(`{"folders":[{"name":"old","path":"repos/removed"}],"settings":{"editor.tabSize":2},"tasks":[]}`)
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := testEnv().syncVSCodeWorkspace(root, ProjectConfig{Name: "empty-project", Repos: map[string]RepoConfig{}}); err != nil {
		t.Fatal(err)
	}
	data, workspace := readVSCodeWorkspace(t, root)
	want := []vscodeWorkspaceFolder{{Name: "empty-project", Path: "."}}
	if len(workspace.Folders) != 1 || workspace.Folders[0] != want[0] {
		t.Fatalf("empty repo workspace folders = %#v, want %#v", workspace.Folders, want)
	}
	if bytes.Contains(data, []byte("settings")) || bytes.Contains(data, []byte("tasks")) || bytes.Contains(data, []byte("removed")) {
		t.Fatalf("sync did not replace the whole workspace file: %s", data)
	}
}

func TestAutoSyncVSCodeWorkspaceDefaultsOnAndFalseSkipsEverything(t *testing.T) {
	root := newVSCodeTestRepo(t)
	cfg := ProjectConfig{Name: "default-on", Repos: map[string]RepoConfig{}}
	if err := testEnv().autoSyncVSCodeWorkspace(root, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, vscodeWorkspaceFile)); err != nil {
		t.Fatalf("default auto-sync did not generate workspace: %v", err)
	}
	if data, err := os.ReadFile(vscodeTestExcludePath(t, root)); err != nil || !hasVSCodeExcludeRule(data) {
		t.Fatalf("default auto-sync did not update exclude: %v", err)
	}

	plainRoot := t.TempDir()
	workspacePath := filepath.Join(plainRoot, vscodeWorkspaceFile)
	old := []byte("keep existing workspace\n")
	if err := os.WriteFile(workspacePath, old, 0600); err != nil {
		t.Fatal(err)
	}
	falseValue := false
	cfg.VSCode.AutoSync = &falseValue
	if err := testEnv().autoSyncVSCodeWorkspace(plainRoot, cfg); err != nil {
		t.Fatalf("disabled auto-sync should skip even outside a Git repo: %v", err)
	}
	got, err := os.ReadFile(workspacePath)
	if err != nil || !bytes.Equal(got, old) {
		t.Fatalf("disabled auto-sync changed existing workspace: %v, %q", err, got)
	}
	if _, err := os.Lstat(filepath.Join(plainRoot, ".git")); !os.IsNotExist(err) {
		t.Fatalf("disabled auto-sync unexpectedly touched Git state: %v", err)
	}
}

func TestSyncVSCodeWorkspacePreservesExcludeBytesAndIsIdempotent(t *testing.T) {
	root := newVSCodeTestRepo(t)
	excludePath := vscodeTestExcludePath(t, root)
	original := []byte("# existing rule\r\ncustom-pattern")
	if err := os.WriteFile(excludePath, original, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := ProjectConfig{Name: "project", Repos: map[string]RepoConfig{}}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append(append([]byte(nil), original...), '\n'), []byte("/"+vscodeWorkspaceFile+"\n")...)
	if !bytes.Equal(first, want) {
		t.Fatalf("exclude bytes changed unexpectedly:\n got %q\nwant %q", first, want)
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(excludePath)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("repeat sync changed exclude: %v\n first %q\nsecond %q", err, first, second)
	}
	if bytes.Count(second, []byte("/"+vscodeWorkspaceFile)) != 1 {
		t.Fatalf("exclude rule should appear once: %q", second)
	}
}

func TestSyncVSCodeWorkspaceRejectsUnsafeTargets(t *testing.T) {
	root := newVSCodeTestRepo(t)
	cfg := ProjectConfig{Name: "project", Repos: map[string]RepoConfig{}}

	outside := filepath.Join(t.TempDir(), "outside")
	outsideContent := []byte("external data")
	if err := os.WriteFile(outside, outsideContent, 0600); err != nil {
		t.Fatal(err)
	}
	workspacePath := filepath.Join(root, vscodeWorkspaceFile)
	if err := os.Symlink(outside, workspacePath); err != nil {
		t.Fatal(err)
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("workspace symlink should be rejected with context: %v", err)
	}
	if got, err := os.ReadFile(outside); err != nil || !bytes.Equal(got, outsideContent) {
		t.Fatalf("workspace symlink target changed: %v, %q", err, got)
	}
	if err := os.Remove(workspacePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspacePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("workspace directory should be rejected with context: %v", err)
	}
	if err := os.Remove(workspacePath); err != nil {
		t.Fatal(err)
	}

	excludePath := vscodeTestExcludePath(t, root)
	backupPath := excludePath + ".test-backup"
	if err := os.Rename(excludePath, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, excludePath); err != nil {
		t.Fatal(err)
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err == nil || !strings.Contains(err.Error(), "exclude") {
		t.Fatalf("exclude symlink should be rejected with context: %v", err)
	}
	if got, err := os.ReadFile(outside); err != nil || !bytes.Equal(got, outsideContent) {
		t.Fatalf("exclude symlink target changed: %v, %q", err, got)
	}
	if err := os.Remove(excludePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(excludePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err == nil || !strings.Contains(err.Error(), "exclude") {
		t.Fatalf("exclude directory should be rejected with context: %v", err)
	}
	if got, err := os.ReadFile(outside); err != nil || !bytes.Equal(got, outsideContent) {
		t.Fatalf("rejecting unsafe targets changed external data: %v, %q", err, got)
	}
}

func TestSyncVSCodeWorkspaceCanRetryAfterWriteBlockIsRemoved(t *testing.T) {
	root := newVSCodeTestRepo(t)
	path := filepath.Join(root, vscodeWorkspaceFile)
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := ProjectConfig{Name: "retry", Repos: map[string]RepoConfig{}}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err == nil {
		t.Fatal("expected workspace write to fail while target is a directory")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := testEnv().syncVSCodeWorkspace(root, cfg); err != nil {
		t.Fatalf("sync should succeed after the write block is removed: %v", err)
	}
	_, workspace := readVSCodeWorkspace(t, root)
	if len(workspace.Folders) != 1 || workspace.Folders[0].Name != "retry" {
		t.Fatalf("retry wrote unexpected workspace: %#v", workspace.Folders)
	}
}
