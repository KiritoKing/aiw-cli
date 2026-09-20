package aiw

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (f *fixture) lifecycleScripts() string {
	f.t.Helper()
	log := filepath.Join(f.base, "lifecycle.log")
	f.t.Setenv("AIW_TEST_LIFECYCLE_LOG", log)
	f.t.Setenv("AIW_TEST_SETUP_FAIL", filepath.Join(f.base, "setup-fail"))
	f.t.Setenv("AIW_TEST_CLEANUP_FAIL", filepath.Join(f.base, "cleanup-fail"))
	f.t.Setenv("AIW_TEST_CLEANUP_DIRTY", filepath.Join(f.base, "cleanup-dirty"))
	f.t.Setenv("AIW_TEST_CLEANUP_COMMIT", filepath.Join(f.base, "cleanup-commit"))
	f.t.Setenv("AIW_TEST_SETUP_DIRTY", filepath.Join(f.base, "setup-dirty"))
	for _, name := range []string{"web", "api"} {
		scripts := filepath.Join(f.source[name], "scripts")
		if err := os.MkdirAll(scripts, 0755); err != nil {
			f.t.Fatal(err)
		}
		setup := `#!/bin/sh
test -L "$AIW_CHANGE_ROOT/repos/$AIW_REPO_NAME" || exit 12
printf 'setup:%s\n' "$AIW_REPO_NAME" >> "$AIW_TEST_LIFECYCLE_LOG"
if test -e "$AIW_TEST_SETUP_FAIL" && test "$AIW_REPO_NAME" = web; then exit 7; fi
if test -e "$AIW_TEST_SETUP_DIRTY"; then printf 'local\n' > setup-output.txt; fi
`
		cleanup := `#!/bin/sh
printf 'cleanup:%s\n' "$AIW_REPO_NAME" >> "$AIW_TEST_LIFECYCLE_LOG"
if test -e "$AIW_TEST_CLEANUP_FAIL" && test "$AIW_REPO_NAME" = web; then exit 8; fi
if test -e "$AIW_TEST_CLEANUP_DIRTY"; then printf 'generated\n' > cleanup-output.txt; fi
if test -e "$AIW_TEST_SETUP_DIRTY" && test -e setup-output.txt; then unlink setup-output.txt; fi
if test -e "$AIW_TEST_CLEANUP_COMMIT"; then
  printf 'committed\n' > cleanup-commit.txt
  git add cleanup-commit.txt
  git -c user.name='AIW Test' -c user.email='aiw-test@example.com' commit -m cleanup
fi
`
		if err := os.WriteFile(filepath.Join(scripts, "setup.sh"), []byte(setup), 0755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(scripts, "cleanup.sh"), []byte(cleanup), 0755); err != nil {
			f.t.Fatal(err)
		}
		f.mustGit(f.source[name], "add", "scripts")
		f.mustGit(f.source[name], "commit", "-m", "lifecycle scripts")
		f.mustGit(f.source[name], "push", "origin", "main")
	}
	cfg, err := loadProject(f.root)
	if err != nil {
		f.t.Fatal(err)
	}
	for name, repo := range cfg.Repos {
		repo.Setup = &RepoCommand{Command: "./scripts/setup.sh"}
		repo.Cleanup = &RepoCommand{Command: "./scripts/cleanup.sh"}
		cfg.Repos[name] = repo
	}
	if err := saveProject(f.root, cfg); err != nil {
		f.t.Fatal(err)
	}
	f.mustGit(f.root, "add", "aiw.yaml")
	f.mustGit(f.root, "commit", "-m", "configure lifecycle")
	f.mustGit(f.root, "push", "origin", "main")
	return log
}

func lifecycleLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSetupRunsOnEveryMaterializeAndExplicitRetry(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(scanRepos(f.root, os.Getenv("AIW_HOME"), f.base))
	f.checkout()
	c := f.context()
	f.spec("web", "api")
	f.mustRun(c.materialize(false))
	if got := lifecycleLog(t, log); got != "setup:api\nsetup:web\n" {
		t.Fatalf("setup order or links incorrect: %q", got)
	}
	f.mustRun(c.materialize(false))
	if got := strings.Count(lifecycleLog(t, log), "setup:"); got != 4 {
		t.Fatalf("expected all setups to rerun, got %d", got)
	}
	if err := os.WriteFile(os.Getenv("AIW_TEST_SETUP_FAIL"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.materialize(false); err == nil || !strings.Contains(err.Error(), "setup failed") {
		t.Fatalf("expected setup failure, got %v", err)
	}
	s, err := c.state()
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != "setup-pending" || s.Repos["web"].Setup != "failed" || s.Repos["api"].Setup != "success" {
		t.Fatalf("incorrect setup failure state: %+v", s)
	}
	if _, err := os.Readlink(c.linkPath("web")); err != nil {
		t.Fatalf("failed setup removed the worktree link: %v", err)
	}
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	statusErr := c.status(true)
	os.Stdout = oldStdout
	writer.Close()
	output, readErr := io.ReadAll(reader)
	reader.Close()
	if statusErr != nil || readErr != nil {
		t.Fatalf("status read failed: %v, %v", statusErr, readErr)
	}
	var rows []StatusRow
	if err := json.Unmarshal(output, &rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].Ready || rows[0].Phase != "setup-pending" {
		t.Fatalf("status JSON incorrectly reports readiness: %+v", rows[0])
	}
	if err := os.Remove(os.Getenv("AIW_TEST_SETUP_FAIL")); err != nil {
		t.Fatal(err)
	}
	f.mustRun(c.setup(nil))
	s, err = c.state()
	if err != nil || s.Phase != "ready" || s.Repos["web"].Setup != "success" {
		t.Fatalf("setup retry did not make Change ready: %+v, %v", s, err)
	}
	if got := strings.Count(lifecycleLog(t, log), "setup:api"); got != 3 {
		t.Fatalf("retry reran already successful api setup: %d", got)
	}
	f.mustRun(c.setup([]string{"api"}))
	if got := strings.Count(lifecycleLog(t, log), "setup:api"); got != 4 {
		t.Fatalf("explicit repo setup did not run: %d", got)
	}
}

func TestLifecycleDefinitionUsesCommittedBaseline(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	cfg, err := loadProject(f.change)
	if err != nil {
		t.Fatal(err)
	}
	repo := cfg.Repos["web"]
	repo.Setup = nil
	cfg.Repos["web"] = repo
	if err := saveProject(f.change, cfg); err != nil {
		t.Fatal(err)
	}
	c, err = currentChange()
	if err != nil {
		t.Fatal(err)
	}
	f.mustRun(c.materialize(false))
	if got := lifecycleLog(t, log); got != "setup:web\n" {
		t.Fatalf("Change edit bypassed baseline setup: %q", got)
	}
}

func TestCleanupFailureAndCloseFromControlPlane(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "api", f.source["api"]))
	f.checkout()
	c := f.context()
	f.spec("web", "api")
	f.mustRun(c.materialize(false))
	if err := os.WriteFile(os.Getenv("AIW_TEST_CLEANUP_FAIL"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	f.spec("api")
	if err := c.materialize(false); err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected cleanup failure on removal, got %v", err)
	}
	if _, err := os.Readlink(c.linkPath("web")); err != nil {
		t.Fatalf("cleanup failure released web: %v", err)
	}
	if err := os.Remove(os.Getenv("AIW_TEST_CLEANUP_FAIL")); err != nil {
		t.Fatal(err)
	}
	f.mustRun(c.cleanup("web"))
	f.mustRun(c.materialize(false))
	if _, err := os.Lstat(c.linkPath("web")); !os.IsNotExist(err) {
		t.Fatalf("web was not released after cleanup retry: %v", err)
	}
	if got := strings.Count(lifecycleLog(t, log), "cleanup:web"); got != 2 {
		t.Fatalf("successful cleanup reran during release: %d", got)
	}
	if err := os.WriteFile(os.Getenv("AIW_TEST_CLEANUP_DIRTY"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.close(CloseOptions{AllowUnpushed: true}); err == nil || !strings.Contains(err.Error(), "--discard-changes") {
		t.Fatalf("cleanup-created dirty file should require new confirmation: %v", err)
	}
	if _, err := os.Stat(c.Root); err != nil {
		t.Fatalf("control-plane root worktree was removed: %v", err)
	}
	f.mustRun(c.close(CloseOptions{AllowUnpushed: true, Discard: true}))
	if got := strings.Count(lifecycleLog(t, log), "cleanup:api"); got != 1 {
		t.Fatalf("successful close cleanup reran: %d", got)
	}
}

func TestRepairDoesNotExecuteSetup(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	s, err := c.state()
	if err != nil {
		t.Fatal(err)
	}
	s.Phase = "materializing"
	f.mustRun(saveState(c.Home, s))
	registry, err := loadRegistry(c.Home)
	if err != nil {
		t.Fatal(err)
	}
	store, err := storeFor(c.Config, registry, "web")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(c.worktreePath("web")), 0755); err != nil {
		t.Fatal(err)
	}
	f.mustGit(store, "worktree", "add", "-b", c.Branch, c.worktreePath("web"), "refs/remotes/origin/main")
	f.mustRun(c.repair())
	if got := lifecycleLog(t, log); got != "" {
		t.Fatalf("repair executed setup: %q", got)
	}
	s, err = c.state()
	if err != nil || s.Phase != "setup-pending" {
		t.Fatalf("repair should leave setup pending: %+v, %v", s, err)
	}
	f.mustRun(c.setup(nil))
	if got := lifecycleLog(t, log); got != "setup:web\n" {
		t.Fatalf("explicit setup did not run: %q", got)
	}
	s, err = c.state()
	if err != nil {
		t.Fatal(err)
	}
	entry := s.Repos["web"]
	entry.Setup = "running"
	s.Repos["web"] = entry
	f.mustRun(saveState(c.Home, s))
	f.mustRun(c.repair())
	if got := lifecycleLog(t, log); got != "setup:web\n" {
		t.Fatalf("repair replayed an interrupted setup: %q", got)
	}
	if err := c.materialize(false); err == nil || !strings.Contains(err.Error(), "aiw change setup web") {
		t.Fatalf("materialize replayed interrupted setup: %v", err)
	}
	if err := c.setup(nil); err == nil || !strings.Contains(err.Error(), "aiw change setup web") {
		t.Fatalf("default setup replayed interrupted setup: %v", err)
	}
	if got := lifecycleLog(t, log); got != "setup:web\n" {
		t.Fatalf("interrupted setup was replayed: %q", got)
	}
	f.mustRun(c.setup([]string{"web"}))
	if got := lifecycleLog(t, log); got != "setup:web\nsetup:web\n" {
		t.Fatalf("explicit retry did not resume interrupted setup: %q", got)
	}
}

func TestCloseCleanupFailureKeepsBusinessAndRootWorktrees(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	f.mustRun(c.materialize(false))
	if err := os.WriteFile(os.Getenv("AIW_TEST_CLEANUP_FAIL"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.close(CloseOptions{AllowUnpushed: true}); err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected close cleanup failure, got %v", err)
	}
	if _, err := os.Stat(c.Root); err != nil {
		t.Fatalf("control-plane root removed: %v", err)
	}
	if _, err := os.Stat(c.worktreePath("web")); err != nil {
		t.Fatalf("business worktree removed after cleanup failure: %v", err)
	}
	if err := os.Remove(os.Getenv("AIW_TEST_CLEANUP_FAIL")); err != nil {
		t.Fatal(err)
	}
	f.mustRun(c.cleanup("web"))
	f.mustRun(c.close(CloseOptions{AllowUnpushed: true}))
	if got := strings.Count(lifecycleLog(t, log), "cleanup:web"); got != 2 {
		t.Fatalf("cleanup should resume once and not replay on close: %d", got)
	}
}

func TestCloseRechecksRemoteAfterCleanupCommit(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	f.mustRun(c.materialize(false))
	f.mustGit(c.Root, "add", ".aiw/change.yaml")
	f.mustGit(c.Root, "commit", "-m", "selection")
	f.mustGit(c.Root, "push", "-u", "origin", c.Branch)
	f.mustGit(c.worktreePath("web"), "push", "-u", "origin", c.Branch)
	if err := os.WriteFile(os.Getenv("AIW_TEST_CLEANUP_COMMIT"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.close(CloseOptions{}); err == nil || !strings.Contains(err.Error(), "--allow-unpushed") {
		t.Fatalf("cleanup commit should require renewed push confirmation: %v", err)
	}
	if _, err := os.Stat(c.worktreePath("web")); err != nil {
		t.Fatalf("close removed worktree with unpushed cleanup commit: %v", err)
	}
	f.mustRun(c.close(CloseOptions{AllowUnpushed: true}))
	if got := strings.Count(lifecycleLog(t, log), "cleanup:web"); got != 1 {
		t.Fatalf("cleanup reran after confirmation: %d", got)
	}
}

func TestExplicitCleanupBeforeDirtyScopeRemoval(t *testing.T) {
	f := newFixture(t)
	log := f.lifecycleScripts()
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "api", f.source["api"]))
	f.checkout()
	c := f.context()
	if err := os.WriteFile(os.Getenv("AIW_TEST_SETUP_DIRTY"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	f.spec("web")
	f.mustRun(c.materialize(false))
	f.spec("api")
	if err := c.materialize(false); err == nil || !strings.Contains(err.Error(), "aiw change cleanup web") {
		t.Fatalf("dirty scope removal should require explicit cleanup: %v", err)
	}
	if got := strings.Count(lifecycleLog(t, log), "cleanup:web"); got != 0 {
		t.Fatalf("dirty scope removal ran cleanup automatically: %d", got)
	}
	f.mustRun(c.cleanup("web"))
	if _, err := os.Stat(filepath.Join(c.worktreePath("web"), "setup-output.txt")); !os.IsNotExist(err) {
		t.Fatalf("cleanup left generated setup file: %v", err)
	}
	f.mustRun(c.materialize(false))
	if _, err := os.Lstat(c.linkPath("web")); !os.IsNotExist(err) {
		t.Fatalf("cleaned worktree was not released: %v", err)
	}
}
