package aiw

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitExistingRepoThroughSymlink(t *testing.T) {
	if _, err := git("", "--version"); err != nil {
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
	if _, err := git("", "init", "-b", "main", realRepo); err != nil {
		t.Fatal(err)
	}
	if err := runInit([]string{filepath.Join(alias, "agent"), "--skip-skills"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(realRepo, "aiw.yaml")); err != nil {
		t.Fatalf("expected AIW config in existing Git root: %v", err)
	}
}
