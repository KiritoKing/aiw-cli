package aiw

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitExistingRepoThroughSymlink(t *testing.T) {
	isolateGitConfig(t)
	if _, err := testEnv().git("", "--version"); err != nil {
		t.Skipf("Git unavailable: %v", err)
	}
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	realRepo := filepath.Join(realDir, "agent")
	if _, err := testEnv().git("", "init", "-b", "main", realRepo); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"init", filepath.Join(alias, "agent"), "--skip-skills"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(realRepo, "aiw.yaml")); err != nil {
		t.Fatalf("expected AIW config in existing Git root: %v", err)
	}
}

func testEnv() commandEnv {
	env := newCommandEnv(context.Background(), defaultIO())
	env.Cwd, _ = os.Getwd()
	return env
}
