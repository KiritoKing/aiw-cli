package aiw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KiritoKing/aiw-cli/internal/gitcli"
	"github.com/KiritoKing/aiw-cli/internal/localfs"
)

func initializeAgentRepo(target string, skipSkills bool) error {
	path, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("refusing to initialize Git in a nonempty directory: %s", path)
		}
		if _, err := gitcli.Run(path, "init", "-b", "main"); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	root, err := gitcli.Root(path)
	if err != nil {
		return fmt.Errorf("inspect Git root at %s: %w", path, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve Agent Repo path %s: %w", path, err)
	}
	if root != resolvedPath {
		return fmt.Errorf("path must be an Agent Repo root: %s (Git root: %s)", path, root)
	}
	if _, err := os.Stat(filepath.Join(path, "aiw.yaml")); err == nil {
		return fmt.Errorf("aiw.yaml already exists: %s", path)
	}
	if err := checkToolchainPaths(path); err != nil {
		return err
	}
	if err := syncAgentInstructions(path); err != nil {
		return err
	}
	b, err := gitcli.Branch(path)
	if err != nil {
		return err
	}
	cfg := ProjectConfig{Version: 1, Name: filepath.Base(path), Base: b, Repos: map[string]RepoConfig{}}
	if err := saveProject(path, cfg); err != nil {
		return err
	}
	ignore := filepath.Join(path, ".gitignore")
	old, err := os.ReadFile(ignore)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !strings.Contains("\n"+string(old)+"\n", "\n/repos/\n") {
		content := string(old)
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "/repos/\n"
		if err := localfs.AtomicWrite(ignore, []byte(content)); err != nil {
			return err
		}
	}
	if err := writeToolchainFiles(path); err != nil {
		return err
	}
	if !skipSkills {
		if err := installSkills(path); err != nil {
			return fmt.Errorf("Agent Repo initialized at %s, but skill installation failed: %w; install skills separately or use your own installer", path, err)
		}
	}
	fmt.Printf("initialized %s; edit aiw.yaml, then commit it in the Agent Repo\n", path)
	return nil
}
