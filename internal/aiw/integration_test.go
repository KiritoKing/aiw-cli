package aiw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	t      *testing.T
	base   string
	root   string
	change string
	source map[string]string
	remote map[string]string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := git("", "--version"); err != nil {
		t.Skipf("Git unavailable: %v", err)
	}
	base, err := os.MkdirTemp("/private/tmp", "aiw-test-")
	if err != nil {
		base = t.TempDir()
	} else {
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	t.Setenv("AIW_HOME", filepath.Join(base, "aiw-home"))
	f := &fixture{t: t, base: base, root: filepath.Join(base, "agent"), change: filepath.Join(base, "change"), source: map[string]string{}, remote: map[string]string{}}
	f.mustRun(Run([]string{"init", f.root, "--skip-skills"}))
	f.configure(f.root)
	f.remote["agent"] = filepath.Join(base, "agent-remote.git")
	f.mustGit("", "init", "--bare", f.remote["agent"])
	f.mustGit(f.root, "remote", "add", "origin", f.remote["agent"])
	for _, name := range []string{"web", "api"} {
		f.source[name] = filepath.Join(base, name)
		f.remote[name] = filepath.Join(base, name+"-remote.git")
		f.mustGit("", "init", "-b", "main", f.source[name])
		f.configure(f.source[name])
		if err := os.WriteFile(filepath.Join(f.source[name], "README.md"), []byte(name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		f.mustGit(f.source[name], "add", "README.md")
		f.mustGit(f.source[name], "commit", "-m", "base")
		f.mustGit("", "init", "--bare", f.remote[name])
		f.mustGit(f.source[name], "remote", "add", "origin", f.remote[name])
		f.mustGit(f.source[name], "push", "-u", "origin", "main")
	}
	cfg := ProjectConfig{Version: 1, Name: "integration", Base: "main", Repos: map[string]RepoConfig{
		"web": {Remote: f.remote["web"], Base: "main"},
		"api": {Remote: f.remote["api"], Base: "main"},
	}}
	if err := saveProject(f.root, cfg); err != nil {
		t.Fatal(err)
	}
	f.mustGit(f.root, "add", "aiw.yaml", ".gitignore", "AGENTS.md")
	f.mustGit(f.root, "commit", "-m", "setup")
	f.mustGit(f.root, "push", "-u", "origin", "main")
	return f
}

func (f *fixture) mustGit(dir string, args ...string) string {
	f.t.Helper()
	out, err := git(dir, args...)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *fixture) mustRun(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) configure(dir string) {
	f.mustGit(dir, "config", "user.name", "AIW Test")
	f.mustGit(dir, "config", "user.email", "aiw-test@example.com")
}

func (f *fixture) checkout() {
	f.mustGit(f.root, "worktree", "add", "-b", "feat/integration", f.change, "main")
}

func (f *fixture) context() ChangeContext {
	f.t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chdir(f.change); err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { os.Chdir(previous) })
	c, err := currentChange()
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

func (f *fixture) spec(repos ...string) {
	f.t.Helper()
	if err := saveSpec(f.change, ChangeSpec{Version: 1, Repos: repos}); err != nil {
		f.t.Fatal(err)
	}
}

func TestExternalWorktreeMaterializeAndReconcile(t *testing.T) {
	f := newFixture(t)
	f.mustRun(scanRepos(f.root, os.Getenv("AIW_HOME"), f.base))
	f.checkout()
	c := f.context()
	if _, err := os.Lstat(c.linkPath("web")); !os.IsNotExist(err) {
		t.Fatalf("control-plane worktree should start without business repos: %v", err)
	}
	f.spec("web")
	f.mustRun(c.materialize(false))
	link, err := os.Readlink(c.linkPath("web"))
	if err != nil || link != c.worktreePath("web") {
		t.Fatalf("wrong business repo link: %q, %v", link, err)
	}
	if got := f.mustGit(link, "branch", "--show-current"); got != "feat/integration" {
		t.Fatalf("branch mismatch: %s", got)
	}
	if got := f.mustGit(f.source["web"], "branch", "--show-current"); got != "main" {
		t.Fatalf("source checkout changed branch: %s", got)
	}
	f.spec("web", "api")
	f.mustRun(c.materialize(false))
	if _, err := os.Readlink(c.linkPath("api")); err != nil {
		t.Fatal(err)
	}
	f.spec("api")
	f.mustRun(c.materialize(false))
	if _, err := os.Lstat(c.linkPath("web")); !os.IsNotExist(err) {
		t.Fatalf("removed repo link remains: %v", err)
	}
	if _, err := os.Stat(c.worktreePath("web")); !os.IsNotExist(err) {
		t.Fatalf("removed worktree remains: %v", err)
	}
	registry, err := loadRegistry(c.Home)
	if err != nil {
		t.Fatal(err)
	}
	store, err := storeFor(c.Config, registry, "web")
	if err != nil {
		t.Fatal(err)
	}
	if !gitOk(store, "show-ref", "--verify", "--quiet", "refs/heads/feat/integration") {
		t.Fatal("removing a worktree should preserve its branch")
	}
	f.spec("api", "web")
	f.mustRun(c.materialize(false))
	if _, err := os.Readlink(c.linkPath("web")); err != nil {
		t.Fatalf("previously removed repo could not be rematerialized: %v", err)
	}
	f.mustRun(c.status(true))
	f.mustGit(c.Root, "add", ".aiw/change.yaml")
	f.mustGit(c.Root, "commit", "-m", "selection")
	f.mustGit(c.Root, "push", "-u", "origin", c.Branch)
	for _, name := range []string{"web", "api"} {
		f.mustGit(c.worktreePath(name), "push", "-u", "origin", c.Branch)
	}
	f.mustRun(c.close(CloseOptions{}))
	if _, err := os.Stat(c.Root); err != nil {
		t.Fatalf("control-plane worktree was removed: %v", err)
	}
}

func TestCloseRequiresExplicitRiskFlags(t *testing.T) {
	f := newFixture(t)
	f.mustRun(registerRepo(f.root, os.Getenv("AIW_HOME"), "web", f.source["web"]))
	f.checkout()
	c := f.context()
	f.spec("web")
	f.mustRun(c.materialize(false))
	if err := os.WriteFile(filepath.Join(c.worktreePath("web"), "untracked.txt"), []byte("unsaved"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := c.close(CloseOptions{}); err == nil || !strings.Contains(err.Error(), "--allow-unpushed") {
		t.Fatalf("unpushed close should require explicit flag: %v", err)
	}
	if err := c.close(CloseOptions{AllowUnpushed: true}); err == nil || !strings.Contains(err.Error(), "--discard-changes") {
		t.Fatalf("dirty close should require separate flag: %v", err)
	}
	f.mustRun(c.close(CloseOptions{AllowUnpushed: true, Discard: true}))
	if _, err := os.Stat(c.Root); err != nil {
		t.Fatalf("control-plane worktree was removed: %v", err)
	}
	if _, err := os.Stat(c.worktreePath("web")); !os.IsNotExist(err) {
		t.Fatalf("AIW worktree remains after close: %v", err)
	}
}

func TestScanDuplicateMatchDoesNotRegister(t *testing.T) {
	f := newFixture(t)
	copyPath := filepath.Join(f.base, "other-web")
	f.mustGit("", "clone", f.remote["web"], copyPath)
	if err := scanRepos(f.root, os.Getenv("AIW_HOME"), f.base); err == nil || !strings.Contains(err.Error(), "multiple local checkouts") {
		t.Fatalf("expected duplicate scan error, got %v", err)
	}
	registry, err := loadRegistry(os.Getenv("AIW_HOME"))
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Repos) != 0 {
		t.Fatalf("scan registered repos before detecting ambiguity: %+v", registry.Repos)
	}
}

func TestScanStopsAtRepositoryBoundary(t *testing.T) {
	f := newFixture(t)
	f.mustGit("", "clone", f.remote["web"], filepath.Join(f.source["web"], "nested-web"))
	f.mustRun(scanRepos(f.root, os.Getenv("AIW_HOME"), f.base))
	registry, err := loadRegistry(os.Getenv("AIW_HOME"))
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Repos) != 2 {
		t.Fatalf("expected only top-level matching repos, got %d", len(registry.Repos))
	}
}

func TestSelectCommitsOnlyAIWManifestWithoutMaterializing(t *testing.T) {
	f := newFixture(t)
	f.checkout()
	c := f.context()
	if err := os.WriteFile(filepath.Join(c.Root, "notes.md"), []byte("other staged work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f.mustGit(c.Root, "add", "notes.md")
	f.mustRun(c.selectRepos([]string{"web"}))
	if got := f.mustGit(c.Root, "show", "--pretty=format:", "--name-only", "HEAD"); got != ".aiw/change.yaml" {
		t.Fatalf("AIW committed unrelated files: %q", got)
	}
	if got := f.mustGit(c.Root, "diff", "--cached", "--name-only"); got != "notes.md" {
		t.Fatalf("unrelated staged file was changed: %q", got)
	}
	if _, err := os.Lstat(c.linkPath("web")); !os.IsNotExist(err) {
		t.Fatalf("selection materialized before Apply: %v", err)
	}
}

func TestRepairAdoptsInterruptedGitWorktree(t *testing.T) {
	f := newFixture(t)
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
	if _, err := os.Readlink(c.linkPath("web")); err != nil {
		t.Fatal(fmt.Errorf("repair did not recreate link: %w", err))
	}
}

func TestCanonicalRemoteMatchesSSHAndHTTPS(t *testing.T) {
	ssh := canonicalRemote("git@github.com:example/web.git")
	https := canonicalRemote("https://github.com/example/web.git")
	if ssh != https {
		t.Fatalf("equivalent remotes differ: %q vs %q", ssh, https)
	}
	if got := canonicalRemote("file:///private/tmp/web.git"); got != "file:/private/tmp/web.git" {
		t.Fatalf("file remote not normalized: %q", got)
	}
}
