package aiw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	agentStart = "<!-- aiw:start -->"
	agentEnd   = "<!-- aiw:end -->"
	agentRules = `## AIW multi-repo Git rules

- The Agent Repo and every business repo have independent Git histories. A commit in one repo does not include changes in another.
- repos/<name> points to an AIW-managed business worktree. The Agent Repo Git status and diff do not show changes inside those worktrees.
- Read aiw.yaml for available repos and .aiw/change.yaml for this Change's selection. Run aiw change materialize after the selection changes and before dispatching business work; check aiw change status --json for readiness.
- Check each business repo's branch, status, diff, tests, and delivery separately (for example, git -C repos/<name> status). Follow that repo's own AGENTS.md and contribution rules.
- Keep commits, pushes, and merge requests in their owning business repos. Record their branch, commit SHA, review link, and verification in the root Change context. Do not treat the root diff as proof of business delivery.
- Inspect local changes before removing a repo or closing a Change. The control plane removes the root worktree only after aiw change close succeeds.`
)

func runAgents(args []string) error {
	if len(args) != 1 || args[0] != "sync" {
		return fmt.Errorf("usage: aiw agents sync")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		return err
	}
	if err := syncAgentInstructions(root); err != nil {
		return err
	}
	fmt.Printf("updated %s\n", filepath.Join(root, "AGENTS.md"))
	return nil
}

func syncAgentInstructions(root string) error {
	path := filepath.Join(root, "AGENTS.md")
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to update non-regular AGENTS.md: %s", path)
	}
	var original []byte
	if err == nil {
		original, err = os.ReadFile(path)
		if err != nil {
			return err
		}
	}
	updated, err := updateAgentBlock(original)
	if err != nil {
		return fmt.Errorf("AGENTS.md: %w", err)
	}
	if bytes.Equal(original, updated) && info != nil {
		return nil
	}
	if err := atomicWrite(path, updated); err != nil {
		return err
	}
	if info != nil {
		return os.Chmod(path, info.Mode().Perm())
	}
	return nil
}

func updateAgentBlock(original []byte) ([]byte, error) {
	start := []byte(agentStart)
	end := []byte(agentEnd)
	startCount, endCount := bytes.Count(original, start), bytes.Count(original, end)
	if (startCount == 0 && endCount != 0) || (startCount != 0 && endCount == 0) || startCount > 1 || endCount > 1 {
		return nil, fmt.Errorf("expected exactly one matched aiw:start / aiw:end pair")
	}
	eol := "\n"
	if bytes.Contains(original, []byte("\r\n")) {
		eol = "\r\n"
	}
	inner := eol + strings.ReplaceAll(agentRules, "\n", eol) + eol
	if startCount == 0 {
		updated := append([]byte(nil), original...)
		if len(updated) > 0 {
			if !bytes.HasSuffix(updated, []byte("\n")) {
				updated = append(updated, eol...)
			}
			updated = append(updated, eol...)
		}
		updated = append(updated, agentStart...)
		updated = append(updated, inner...)
		updated = append(updated, agentEnd...)
		updated = append(updated, eol...)
		return updated, nil
	}
	startAt, endAt := bytes.Index(original, start), bytes.Index(original, end)
	if startAt >= endAt || !standaloneMarker(original, startAt, start) || !standaloneMarker(original, endAt, end) {
		return nil, fmt.Errorf("aiw markers must be ordered and each on its own line")
	}
	updated := make([]byte, 0, len(original)+len(inner))
	updated = append(updated, original[:startAt+len(start)]...)
	updated = append(updated, inner...)
	updated = append(updated, original[endAt:]...)
	return updated, nil
}

func standaloneMarker(data []byte, at int, marker []byte) bool {
	if at > 0 && data[at-1] != '\n' {
		return false
	}
	after := data[at+len(marker):]
	return len(after) == 0 || after[0] == '\n' || bytes.HasPrefix(after, []byte("\r\n"))
}
