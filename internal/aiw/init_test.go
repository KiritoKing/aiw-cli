package aiw

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KiritoKing/aiw-cli/internal/gitcli"
)

func TestInitExistingRepoThroughSymlink(t *testing.T) {
	if _, err := gitcli.Run("", "--version"); err != nil {
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
	if _, err := gitcli.Run("", "init", "-b", "main", realRepo); err != nil {
		t.Fatal(err)
	}
	if err := (Service{}).Init(filepath.Join(alias, "agent"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(realRepo, "aiw.yaml")); err != nil {
		t.Fatalf("expected AIW config in existing Git root: %v", err)
	}
}
