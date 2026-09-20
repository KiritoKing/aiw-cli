package aiw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type ChangeContext struct {
	Root    string
	Project string
	Branch  string
	Config  ProjectConfig
	Home    string
}

func currentChange() (ChangeContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return ChangeContext{}, err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		return ChangeContext{}, err
	}
	actual, err := gitRoot(root)
	if err != nil || actual != root {
		return ChangeContext{}, fmt.Errorf("aiw.yaml must be at the Agent Repo worktree root: %s", root)
	}
	cfg, err := loadProject(root)
	if err != nil {
		return ChangeContext{}, err
	}
	b, err := branch(root)
	if err != nil {
		return ChangeContext{}, err
	}
	if b == cfg.Base {
		return ChangeContext{}, fmt.Errorf("run this command in a control-plane-created Change worktree, not the Agent Repo base branch %s", b)
	}
	if err := assertBranchName(b); err != nil {
		return ChangeContext{}, err
	}
	common, err := gitCommonDir(root)
	if err != nil {
		return ChangeContext{}, err
	}
	home, err := dataHome()
	if err != nil {
		return ChangeContext{}, err
	}
	return ChangeContext{Root: root, Project: common, Branch: b, Config: cfg, Home: home}, nil
}

func (c ChangeContext) worktreePath(name string) string {
	return filepath.Join(c.Home, "repo-worktrees", hash(c.Project), hash(c.Branch), name)
}

func (c ChangeContext) linkPath(name string) string {
	return filepath.Join(c.Root, "repos", name)
}

func (c ChangeContext) state() (ChangeState, error) {
	s, err := loadState(c.Home, c.Project, c.Branch)
	if err != nil {
		return s, err
	}
	if s.Project != "" && (s.Project != c.Project || s.Branch != c.Branch) {
		return s, fmt.Errorf("change state identity mismatch")
	}
	if s.Project == "" {
		s = ChangeState{Project: c.Project, Root: c.Root, Branch: c.Branch, Repos: map[string]RepoState{}, Phase: "new"}
	} else if s.Root != c.Root {
		if _, err := os.Stat(s.Root); err == nil {
			return s, fmt.Errorf("branch %s is already registered at %s", c.Branch, s.Root)
		}
		s.Root = c.Root
	}
	return s, nil
}

func (c ChangeContext) materialize(discard bool) error {
	return c.reconcile(discard, true)
}

func (c ChangeContext) reconcile(discard, runLifecycle bool) error {
	spec, err := loadSpec(c.Root, c.Config)
	if err != nil {
		return err
	}
	lifecycle, err := c.lifecycleConfig()
	if err != nil {
		return err
	}
	tracked, err := git(c.Root, "ls-files", "--", "repos")
	if err != nil {
		return err
	}
	if tracked != "" {
		return fmt.Errorf("repos/ contains tracked Agent Repo files; remove them from Git before materializing")
	}
	for _, name := range spec.Repos {
		if !gitOk(c.Root, "check-ignore", "--quiet", "--no-index", "repos/"+name) {
			return fmt.Errorf("repos/%s is not ignored by the Agent Repo; add /repos/ to .gitignore", name)
		}
	}
	registry, err := loadRegistry(c.Home)
	if err != nil {
		return err
	}
	s, err := c.state()
	if err != nil {
		return err
	}
	desired := map[string]bool{}
	for _, name := range spec.Repos {
		desired[name] = true
		if _, err := c.repoLifecycle(lifecycle, name); err != nil {
			return err
		}
	}
	removed := []string{}
	for name, old := range s.Repos {
		if !desired[name] && old.Path != "" {
			removed = append(removed, name)
		}
	}
	sort.Strings(removed)
	stores := map[string]string{}
	for _, name := range spec.Repos {
		store, err := storeFor(c.Config, registry, name)
		if err != nil {
			return err
		}
		stores[name] = store
		if _, exists := s.Repos[name]; !exists {
			if _, err := os.Lstat(c.worktreePath(name)); err == nil {
				return fmt.Errorf("unregistered worktree path exists: %s", c.worktreePath(name))
			} else if !os.IsNotExist(err) {
				return err
			}
			if _, err := os.Lstat(c.linkPath(name)); err == nil {
				return fmt.Errorf("unregistered link exists: %s", c.linkPath(name))
			} else if !os.IsNotExist(err) {
				return err
			}
			if gitOk(store, "show-ref", "--verify", "--quiet", "refs/heads/"+c.Branch) && s.Phase != "closed" {
				return fmt.Errorf("branch %s already exists in repo %s; use change repair if this is an interrupted AIW operation", c.Branch, name)
			}
		}
	}
	if runLifecycle {
		for _, name := range removed {
			old := s.Repos[name]
			if _, err := c.repoLifecycle(lifecycle, name); err != nil {
				return err
			}
			store := old.Store
			if store == "" {
				store, err = storeFor(c.Config, registry, name)
				if err != nil {
					return err
				}
			}
			if err := c.checkOwned(name, old.Path, store); err != nil {
				return err
			}
			dirty, err := dirtyDetails(old.Path)
			if err != nil {
				return err
			}
			if dirty != "" && !discard {
				return fmt.Errorf("repo %s has local changes; inspect %s or run aiw change cleanup %s explicitly before removing it from the Change", name, old.Path, name)
			}
		}
	}
	s.Phase = "materializing"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	for _, name := range spec.Repos {
		if old, exists := s.Repos[name]; exists && old.Path != "" {
			if err := c.checkOwned(name, old.Path, stores[name]); err != nil {
				return err
			}
			if old.Store == "" {
				old.Store = stores[name]
				s.Repos[name] = old
			}
			continue
		}
		path := c.worktreePath(name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		baseRef := "refs/remotes/origin/" + c.Config.Repos[name].Base
		existingBranch := gitOk(stores[name], "show-ref", "--verify", "--quiet", "refs/heads/"+c.Branch)
		base := s.Repos[name].Base
		if base == "" || !existingBranch {
			base, err = refSHA(stores[name], baseRef)
			if err != nil {
				return fmt.Errorf("repo %s has no local baseline; run aiw repo sync %s: %w", name, name, err)
			}
		}
		args := []string{"worktree", "add", "-b", c.Branch, path, base}
		if existingBranch {
			if s.Phase != "materializing" || s.Repos[name].Base == "" {
				return fmt.Errorf("unowned branch %s in repo %s", c.Branch, name)
			}
			args = []string{"worktree", "add", path, c.Branch}
		}
		if _, err := git(stores[name], args...); err != nil {
			return err
		}
		s.Repos[name] = RepoState{Path: path, Base: base, Store: stores[name], Setup: "pending"}
		if err := saveState(c.Home, s); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(c.linkPath(name)), 0755); err != nil {
			return err
		}
		if err := os.Symlink(path, c.linkPath(name)); err != nil {
			return err
		}
		fmt.Printf("materialized %s -> %s\n", name, path)
	}
	if runLifecycle {
		for _, name := range removed {
			old := s.Repos[name]
			store := old.Store
			if store == "" {
				var err error
				store, err = storeFor(c.Config, registry, name)
				if err != nil {
					return err
				}
			}
			if err := c.cleanupRepo(&s, name, lifecycle, false); err != nil {
				return err
			}
			if err := c.removeWorktree(name, old.Path, store, discard); err != nil {
				return err
			}
			old.Path = ""
			old.Cleanup = "success"
			s.Repos[name] = old
			if err := saveState(c.Home, s); err != nil {
				return err
			}
		}
	}
	if !runLifecycle {
		for _, name := range spec.Repos {
			entry := s.Repos[name]
			repo := lifecycle.Repos[name]
			if repo.Setup == nil && entry.Setup == "" {
				entry.Setup = "success"
				s.Repos[name] = entry
			}
		}
		s.Phase = "setup-pending"
		if selectionReady(spec.Repos, s) {
			s.Phase = "ready"
		}
		return saveState(c.Home, s)
	}
	s.Phase = "setup-pending"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	names := append([]string(nil), spec.Repos...)
	sort.Strings(names)
	for _, name := range names {
		if s.Repos[name].Setup == "running" {
			return fmt.Errorf("repo %s setup was interrupted; run aiw change setup %s to retry explicitly", name, name)
		}
	}
	for _, name := range names {
		entry := s.Repos[name]
		if entry.Setup == "success" {
			entry.Setup = "pending"
		}
		entry.Cleanup = ""
		s.Repos[name] = entry
		if err := saveState(c.Home, s); err != nil {
			return err
		}
		if err := c.setupRepo(&s, name, lifecycle); err != nil {
			return err
		}
	}
	s.Phase = "ready"
	if err := saveState(c.Home, s); err != nil {
		return err
	}
	return nil
}

func (c ChangeContext) checkOwned(name, path, store string) error {
	if path != c.worktreePath(name) {
		return fmt.Errorf("worktree path mismatch for %s: %s", name, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed worktree path is not a directory: %s", path)
	}
	actual, err := gitCommonDir(path)
	if err != nil {
		return err
	}
	storeReal, err := filepath.EvalSymlinks(store)
	if err != nil || actual != storeReal {
		return fmt.Errorf("worktree %s is not owned by managed store %s", path, store)
	}
	b, err := branch(path)
	if err != nil || b != c.Branch {
		return fmt.Errorf("worktree %s is not on expected branch %s", path, c.Branch)
	}
	link, err := os.Readlink(c.linkPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(c.linkPath(name)), 0755); err != nil {
				return err
			}
			return os.Symlink(path, c.linkPath(name))
		}
		return err
	}
	if link != path {
		return fmt.Errorf("link %s points outside registered worktree", c.linkPath(name))
	}
	return nil
}

func (c ChangeContext) removeWorktree(name, path, store string, discard bool) error {
	if err := c.checkOwned(name, path, store); err != nil {
		return err
	}
	details, err := dirtyDetails(path)
	if err != nil {
		return err
	}
	if details != "" && !discard {
		return fmt.Errorf("repo %s has local changes; inspect %s or explicitly pass --discard-changes", name, path)
	}
	args := []string{"worktree", "remove"}
	if details != "" {
		args = append(args, "--force")
	}
	args = append(args, path)
	if _, err := git(store, args...); err != nil {
		return err
	}
	if err := os.Remove(c.linkPath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type StatusRow struct {
	Repo    string    `json:"repo"`
	Path    string    `json:"path"`
	State   GitStatus `json:"git"`
	BaseSHA string    `json:"baseSha,omitempty"`
	Drift   bool      `json:"drift"`
	Active  bool      `json:"active"`
	Phase   string    `json:"phase,omitempty"`
	Ready   bool      `json:"ready"`
	Setup   string    `json:"setup,omitempty"`
	Cleanup string    `json:"cleanup,omitempty"`
}

func (c ChangeContext) status(jsonOutput bool) error {
	s, err := c.state()
	if err != nil {
		return err
	}
	lifecycle, err := c.lifecycleConfig()
	if err != nil {
		return err
	}
	rootStatus, err := status(c.Root)
	if err != nil {
		return err
	}
	ready := s.Phase == "ready"
	if ready {
		spec, err := loadSpec(c.Root, c.Config)
		ready = err == nil && selectionReady(spec.Repos, s)
	}
	rows := []StatusRow{{Repo: "agent-repo", Path: c.Root, State: rootStatus, Drift: rootStatus.Branch != c.Branch, Active: true, Phase: s.Phase, Ready: ready}}
	names := make([]string, 0, len(s.Repos))
	for name := range s.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := s.Repos[name].Path
		if p == "" {
			continue
		}
		gitState, err := status(p)
		if err != nil {
			return fmt.Errorf("repo %s: %w", name, err)
		}
		setup := s.Repos[name].Setup
		if setup == "" && lifecycle.Repos[name].Setup == nil {
			setup = "success"
		}
		rows = append(rows, StatusRow{Repo: name, Path: p, State: gitState, BaseSHA: s.Repos[name].Base, Drift: gitState.Branch != c.Branch, Active: true, Ready: setup == "success", Setup: setup, Cleanup: s.Repos[name].Cleanup})
	}
	if jsonOutput {
		out, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	for _, row := range rows {
		label := "clean"
		if row.State.Dirty {
			label = fmt.Sprintf("dirty(%d)", row.State.Changed)
		}
		if row.Drift {
			label += " DRIFT"
		}
		if row.Repo == "agent-repo" {
			label += " phase=" + row.Phase
			if !row.Ready {
				label += " NOT-READY"
			}
		} else {
			label += " setup=" + row.Setup
			if row.Cleanup == "failed" || row.Cleanup == "running" {
				label += " cleanup=" + row.Cleanup
			}
		}
		fmt.Printf("%-16s %-24s %-10s %s\n", row.Repo, row.State.Branch, label, row.Path)
	}
	return nil
}

func (c ChangeContext) updateSelection(action, name string) error {
	if _, ok := c.Config.Repos[name]; !ok {
		return fmt.Errorf("unknown repo %s", name)
	}
	spec := ChangeSpec{Version: 1}
	if _, err := os.Stat(specPath(c.Root)); err == nil {
		var loadErr error
		spec, loadErr = loadSpec(c.Root, c.Config)
		if loadErr != nil {
			return loadErr
		}
	}
	selected := map[string]bool{}
	for _, existing := range spec.Repos {
		selected[existing] = true
	}
	if action == "add" {
		selected[name] = true
	} else {
		delete(selected, name)
	}
	if len(selected) == 0 {
		return fmt.Errorf("change must select at least one repo")
	}
	spec.Repos = spec.Repos[:0]
	for item := range selected {
		spec.Repos = append(spec.Repos, item)
	}
	sort.Strings(spec.Repos)
	if err := c.selectRepos(spec.Repos); err != nil {
		return err
	}
	s, err := c.state()
	if err != nil {
		return err
	}
	if s.Phase == "ready" {
		return c.materialize(false)
	}
	return nil
}

func (c ChangeContext) selectRepos(names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("at least one repo is required")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if _, ok := c.Config.Repos[name]; !ok {
			return fmt.Errorf("unknown repo %s", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate repo %s", name)
		}
		seen[name] = true
	}
	sort.Strings(names)
	previous, _ := os.ReadFile(specPath(c.Root))
	if err := saveSpec(c.Root, ChangeSpec{Version: 1, Repos: names}); err != nil {
		return err
	}
	current, err := os.ReadFile(specPath(c.Root))
	if err != nil {
		return err
	}
	if string(previous) == string(current) {
		return nil
	}
	if _, err := git(c.Root, "add", "--", ".aiw/change.yaml"); err != nil {
		return err
	}
	if _, err := git(c.Root, "commit", "-m", "aiw: update repository selection", "--only", "--", ".aiw/change.yaml"); err != nil {
		return err
	}
	return nil
}

func (c ChangeContext) repair() error {
	s, err := c.state()
	if err != nil {
		return err
	}
	if s.Phase == "closing" {
		for name, entry := range s.Repos {
			if entry.Path != "" {
				if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
					_ = os.Remove(c.linkPath(name))
					entry.Path = ""
					s.Repos[name] = entry
				}
			}
		}
		return saveState(c.Home, s)
	}
	spec, err := loadSpec(c.Root, c.Config)
	if err != nil {
		return err
	}
	registry, err := loadRegistry(c.Home)
	if err != nil {
		return err
	}
	for _, name := range spec.Repos {
		if old, ok := s.Repos[name]; ok && old.Path != "" {
			continue
		}
		path := c.worktreePath(name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		store, err := storeFor(c.Config, registry, name)
		if err != nil {
			return err
		}
		actual, err := gitCommonDir(path)
		if err != nil {
			return err
		}
		storeReal, err := filepath.EvalSymlinks(store)
		if err != nil || actual != storeReal {
			return fmt.Errorf("cannot adopt worktree %s: store mismatch", path)
		}
		b, err := branch(path)
		if err != nil || b != c.Branch {
			return fmt.Errorf("cannot adopt worktree %s: branch mismatch", path)
		}
		base := s.Repos[name].Base
		if base == "" {
			base, _ = refSHA(store, "refs/remotes/origin/"+c.Config.Repos[name].Base)
		}
		s.Repos[name] = RepoState{Path: path, Base: base, Store: store}
		if err := saveState(c.Home, s); err != nil {
			return err
		}
	}
	desired := map[string]bool{}
	for _, name := range spec.Repos {
		desired[name] = true
	}
	for name, entry := range s.Repos {
		if desired[name] || entry.Path == "" {
			continue
		}
		if _, err := os.Lstat(entry.Path); os.IsNotExist(err) {
			_ = os.Remove(c.linkPath(name))
			entry.Path = ""
			s.Repos[name] = entry
			if err := saveState(c.Home, s); err != nil {
				return err
			}
		}
	}
	return c.reconcile(false, false)
}
