package aiw

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

type CloseOptions struct {
	AllowUnpushed bool
	Discard       bool
}

type closeCheck struct {
	Name     string
	Path     string
	Store    string
	Unpushed string
	Dirty    string
}

func (c ChangeContext) close(options CloseOptions) error {
	s, err := c.state()
	if err != nil {
		return err
	}
	if s.Project == "" || s.Phase == "new" {
		return fmt.Errorf("this Change has not been materialized")
	}
	if s.Phase == "closed" {
		fmt.Println("already closed; the control plane still owns the Agent Repo worktree")
		return nil
	}
	registry, err := loadRegistry(c.Home)
	if err != nil {
		return err
	}
	lifecycle, err := c.lifecycleConfig()
	if err != nil {
		return err
	}
	checks := []closeCheck{{Name: "agent-repo", Path: c.Root}}
	names := make([]string, 0, len(s.Repos))
	for name := range s.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := s.Repos[name]
		if entry.Path == "" {
			continue
		}
		store := entry.Store
		if store == "" {
			var err error
			store, err = storeFor(c.Config, registry, name)
			if err != nil {
				return err
			}
		}
		if err := c.checkOwned(name, entry.Path, store); err != nil {
			return err
		}
		if _, err := c.repoLifecycle(lifecycle, name); err != nil {
			return err
		}
		if entry.Store == "" {
			entry.Store = store
			s.Repos[name] = entry
		}
		checks = append(checks, closeCheck{Name: name, Path: entry.Path, Store: store})
	}
	for i := range checks {
		local, err := head(checks[i].Path)
		if err != nil {
			checks[i].Unpushed = "local HEAD unavailable: " + err.Error()
			continue
		}
		remote, err := remoteHead(checks[i].Path, c.Branch)
		if err != nil {
			checks[i].Unpushed = err.Error()
		} else if remote != local {
			checks[i].Unpushed = fmt.Sprintf("local %s differs from remote %s", local, remote)
		}
		if checks[i].Name != "agent-repo" {
			checks[i].Dirty, err = dirtyDetails(checks[i].Path)
			if err != nil {
				return err
			}
		}
	}
	needsUnpushed, needsDiscard := false, false
	for _, check := range checks {
		if check.Unpushed != "" {
			needsUnpushed = true
			fmt.Fprintf(os.Stderr, "[not confirmed pushed] %s: %s\n", check.Name, check.Unpushed)
		}
		if check.Dirty != "" {
			needsDiscard = true
			fmt.Fprintf(os.Stderr, "[will discard] %s (%s):\n%s\n", check.Name, check.Path, check.Dirty)
		}
	}
	if needsUnpushed && !options.AllowUnpushed {
		if err := confirmRisk(c.Branch, "Remote push is unverified for at least one branch. Close anyway?", "--allow-unpushed"); err != nil {
			return err
		}
	}
	if needsDiscard && !options.Discard {
		if err := confirmRisk(c.Branch, "Local files in AIW business worktrees will be deleted. Close anyway?", "--discard-changes"); err != nil {
			return err
		}
	}
	s.Phase = "closing"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	for _, check := range checks[1:] {
		if err := c.cleanupRepo(&s, check.Name, lifecycle, false); err != nil {
			return fmt.Errorf("close stopped at %s; worktree remains: %w", check.Name, err)
		}
		unpushedAfterCleanup := ""
		local, err := head(check.Path)
		if err != nil {
			unpushedAfterCleanup = "local HEAD unavailable: " + err.Error()
		} else if remote, err := remoteHead(check.Path, c.Branch); err != nil {
			unpushedAfterCleanup = err.Error()
		} else if remote != local {
			unpushedAfterCleanup = fmt.Sprintf("local %s differs from remote %s", local, remote)
		}
		if unpushedAfterCleanup != check.Unpushed && unpushedAfterCleanup != "" && !options.AllowUnpushed {
			fmt.Fprintf(os.Stderr, "[not confirmed pushed after cleanup] %s: %s\n", check.Name, unpushedAfterCleanup)
			if err := confirmRisk(c.Branch, "Cleanup changed the branch or its remote state. Close anyway?", "--allow-unpushed"); err != nil {
				return err
			}
		}
		afterCleanup, err := dirtyDetails(check.Path)
		if err != nil {
			return err
		}
		if afterCleanup != check.Dirty && afterCleanup != "" && !options.Discard {
			fmt.Fprintf(os.Stderr, "[will discard after cleanup] %s (%s):\n%s\n", check.Name, check.Path, afterCleanup)
			if err := confirmRisk(c.Branch, "Cleanup changed files that will be deleted. Close anyway?", "--discard-changes"); err != nil {
				return err
			}
		}
		if err := c.removeWorktree(check.Name, check.Path, check.Store, afterCleanup != ""); err != nil {
			return fmt.Errorf("close stopped at %s; previous removals remain recorded: %w", check.Name, err)
		}
		entry := s.Repos[check.Name]
		entry.Path = ""
		s.Repos[check.Name] = entry
		if err := saveState(c.Home, s); err != nil {
			return err
		}
		fmt.Printf("released %s\n", check.Name)
	}
	s.Phase = "closed"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	fmt.Printf("closed %s; Agent Repo worktree remains at %s for its control plane\n", c.Branch, c.Root)
	return nil
}

func confirmRisk(change, prompt, flag string) error {
	info, err := os.Stdin.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("confirmation required; inspect the warning and rerun with %s", flag)
	}
	fmt.Fprintf(os.Stderr, "%s Type the Change branch %q to confirm: ", prompt, change)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("confirmation unavailable; inspect the warning and rerun with %s: %w", flag, err)
	}
	if strings.TrimSpace(answer) != change {
		return fmt.Errorf("close cancelled")
	}
	return nil
}
