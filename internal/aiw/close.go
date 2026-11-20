package aiw

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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
		fmt.Fprintln(c.Out, "already closed; the control plane still owns the Agent Repo worktree")
		return nil
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
		store, err := c.stateStore(name, entry)
		if err != nil {
			return err
		}
		if err := c.checkOwned(name, entry.Path, store); err != nil {
			return err
		}
		if entry.Store == "" {
			entry.Store = store
			s.Repos[name] = entry
		}
		checks = append(checks, closeCheck{Name: name, Path: entry.Path, Store: store})
	}
	for i := range checks {
		local, err := c.head(checks[i].Path)
		if err != nil {
			checks[i].Unpushed = "local HEAD unavailable: " + err.Error()
			continue
		}
		remote, err := c.remoteHead(checks[i].Path, c.Branch)
		if err != nil {
			checks[i].Unpushed = err.Error()
		} else if remote != local {
			checks[i].Unpushed = fmt.Sprintf("local %s differs from remote %s", local, remote)
		}
		if i > 0 {
			checks[i].Dirty, err = c.dirtyDetails(checks[i].Path)
			if err != nil {
				return err
			}
		}
	}
	needsUnpushed, needsDiscard := false, false
	var risks []string
	for _, check := range checks {
		if check.Unpushed != "" {
			needsUnpushed = true
			risks = append(risks, fmt.Sprintf("[not confirmed pushed] %s: %s", check.Name, check.Unpushed))
		}
		if check.Dirty != "" {
			needsDiscard = true
			risks = append(risks, fmt.Sprintf("[will discard] %s (%s):\n%s", check.Name, check.Path, check.Dirty))
		}
	}
	c.showCloseRisks(risks...)
	if needsUnpushed && !options.AllowUnpushed {
		if err := c.confirmRisk(c.Branch, "Remote push is unverified for at least one branch. Close anyway?", "--allow-unpushed"); err != nil {
			return err
		}
	}
	if needsDiscard && !options.Discard {
		if err := c.confirmRisk(c.Branch, "Local files in AIW business worktrees will be deleted. Close anyway?", "--discard-changes"); err != nil {
			return err
		}
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}
	s.Phase = "closing"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	for _, check := range checks[1:] {
		if err := c.cleanupRepo(&s, check.Name, false); err != nil {
			return fmt.Errorf("close stopped at %s; worktree remains: %w", check.Name, err)
		}
		unpushedAfterCleanup := ""
		local, err := c.head(check.Path)
		if err != nil {
			unpushedAfterCleanup = "local HEAD unavailable: " + err.Error()
		} else if remote, err := c.remoteHead(check.Path, c.Branch); err != nil {
			unpushedAfterCleanup = err.Error()
		} else if remote != local {
			unpushedAfterCleanup = fmt.Sprintf("local %s differs from remote %s", local, remote)
		}
		if unpushedAfterCleanup != check.Unpushed && unpushedAfterCleanup != "" && !options.AllowUnpushed {
			c.showCloseRisks(fmt.Sprintf("[not confirmed pushed after cleanup] %s: %s", check.Name, unpushedAfterCleanup))
			if err := c.confirmRisk(c.Branch, "Cleanup changed the branch or its remote state. Close anyway?", "--allow-unpushed"); err != nil {
				return err
			}
		}
		afterCleanup, err := c.dirtyDetails(check.Path)
		if err != nil {
			return err
		}
		if afterCleanup != check.Dirty && afterCleanup != "" && !options.Discard {
			c.showCloseRisks(fmt.Sprintf("[will discard after cleanup] %s (%s):\n%s", check.Name, check.Path, afterCleanup))
			if err := c.confirmRisk(c.Branch, "Cleanup changed files that will be deleted. Close anyway?", "--discard-changes"); err != nil {
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
		fmt.Fprintf(c.Out, "released %s\n", check.Name)
	}
	s.Phase = "closed"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "closed %s; Agent Repo worktree remains at %s for its control plane\n", c.Branch, c.Root)
	return nil
}

func (c ChangeContext) showCloseRisks(risks ...string) {
	if len(risks) == 0 {
		return
	}
	body := strings.Join(risks, "\n")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = terminalLabel(line)
	}
	body = strings.Join(lines, "\n")
	if c.isInteractive() {
		style := lipgloss.NewRenderer(c.ErrOut).NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		if c.colorEnabled() {
			style = style.BorderForeground(lipgloss.Color("3"))
		}
		body = style.Render("Close risks\n" + body)
	}
	fmt.Fprintln(c.ErrOut, body)
}

func (c ChangeContext) confirmRisk(change, prompt, flag string) error {
	if !c.isInteractive() {
		return fmt.Errorf("confirmation required; inspect the warning and rerun with %s", flag)
	}
	var answer string
	field := huh.NewInput().Title(prompt).
		Description(fmt.Sprintf("Type the Change branch %q to confirm.", change)).
		Value(&answer)
	if err := c.runForm(field); err != nil {
		return fmt.Errorf("close cancelled: %w", err)
	}
	if strings.TrimSpace(answer) != change {
		return fmt.Errorf("close cancelled")
	}
	return nil
}
