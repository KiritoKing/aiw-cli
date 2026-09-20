package aiw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Lifecycle commands come from the committed Agent Repo baseline, not a Change edit.
func (c ChangeContext) lifecycleConfig() (ProjectConfig, error) {
	data, err := git(c.Root, "show", "refs/heads/"+c.Config.Base+":aiw.yaml")
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("read committed baseline aiw.yaml: %w", err)
	}
	cfg, err := parseProject([]byte(data))
	if err != nil {
		return cfg, err
	}
	if cfg.Base != c.Config.Base {
		return cfg, fmt.Errorf("Change and baseline aiw.yaml disagree on Agent Repo base branch")
	}
	return cfg, nil
}

func (c ChangeContext) repoLifecycle(cfg ProjectConfig, name string) (RepoConfig, error) {
	repo, ok := cfg.Repos[name]
	if !ok {
		return repo, fmt.Errorf("repo %s is absent from committed baseline aiw.yaml", name)
	}
	if current, ok := c.Config.Repos[name]; ok && (canonicalRemote(current.Remote) != canonicalRemote(repo.Remote) || current.Base != repo.Base) {
		return repo, fmt.Errorf("repo %s differs from committed baseline aiw.yaml", name)
	}
	return repo, nil
}

func (c ChangeContext) executeRepoCommand(name, path, action string, command *RepoCommand) error {
	if command == nil {
		return nil
	}
	bin := command.Command
	if strings.ContainsRune(bin, os.PathSeparator) && !filepath.IsAbs(bin) {
		bin = filepath.Join(path, bin)
	}
	cmd := exec.Command(bin, command.Args...)
	cmd.Dir = path
	cmd.Env = append(os.Environ(),
		"AIW_LIFECYCLE_ACTIVE=1",
		"AIW_LIFECYCLE_ACTION="+action,
		"AIW_CHANGE_ROOT="+c.Root,
		"AIW_CHANGE_BRANCH="+c.Branch,
		"AIW_REPO_NAME="+name,
		"AIW_REPO_PATH="+path,
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	fmt.Printf("%s %s in %s\n", action, name, path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repo %s %s failed: %w", name, action, err)
	}
	return nil
}

func (c ChangeContext) setupRepo(s *ChangeState, name string, cfg ProjectConfig) error {
	entry := s.Repos[name]
	if entry.Path == "" {
		return fmt.Errorf("repo %s has no materialized worktree", name)
	}
	repo, err := c.repoLifecycle(cfg, name)
	if err != nil {
		return err
	}
	if entry.Store == "" {
		registry, err := loadRegistry(c.Home)
		if err != nil {
			return err
		}
		entry.Store, err = storeFor(c.Config, registry, name)
		if err != nil {
			return err
		}
	}
	if entry.Setup == "running" {
		fmt.Fprintf(os.Stderr, "repo %s setup was interrupted; retry may repeat script effects\n", name)
	}
	if err := c.checkOwned(name, entry.Path, entry.Store); err != nil {
		return err
	}
	entry.Setup = "running"
	s.Repos[name] = entry
	if err := saveState(c.Home, *s); err != nil {
		return err
	}
	err = c.executeRepoCommand(name, entry.Path, "setup", repo.Setup)
	if err != nil {
		entry.Setup = "failed"
	} else {
		entry.Setup = "success"
	}
	s.Repos[name] = entry
	if saveErr := saveState(c.Home, *s); saveErr != nil {
		return saveErr
	}
	return err
}

func (c ChangeContext) cleanupRepo(s *ChangeState, name string, cfg ProjectConfig, retry bool) error {
	entry := s.Repos[name]
	if entry.Path == "" {
		return fmt.Errorf("repo %s has no materialized worktree", name)
	}
	if entry.Cleanup == "success" {
		return nil
	}
	if (entry.Cleanup == "failed" || entry.Cleanup == "running") && !retry {
		return fmt.Errorf("repo %s cleanup is %s; run aiw change cleanup %s before retrying release", name, entry.Cleanup, name)
	}
	repo, err := c.repoLifecycle(cfg, name)
	if err != nil {
		return err
	}
	if entry.Store == "" {
		registry, err := loadRegistry(c.Home)
		if err != nil {
			return err
		}
		entry.Store, err = storeFor(c.Config, registry, name)
		if err != nil {
			return err
		}
	}
	if entry.Cleanup == "running" {
		fmt.Fprintf(os.Stderr, "repo %s cleanup was interrupted; retry may repeat script effects\n", name)
	}
	if err := c.checkOwned(name, entry.Path, entry.Store); err != nil {
		return err
	}
	entry.Cleanup = "running"
	s.Repos[name] = entry
	if err := saveState(c.Home, *s); err != nil {
		return err
	}
	err = c.executeRepoCommand(name, entry.Path, "cleanup", repo.Cleanup)
	if err != nil {
		entry.Cleanup = "failed"
	} else {
		entry.Cleanup = "success"
	}
	s.Repos[name] = entry
	if saveErr := saveState(c.Home, *s); saveErr != nil {
		return saveErr
	}
	return err
}

func (c ChangeContext) setup(names []string) error {
	explicit := len(names) == 1
	spec, err := loadSpec(c.Root, c.Config)
	if err != nil {
		return err
	}
	s, err := c.state()
	if err != nil {
		return err
	}
	if s.Phase == "new" || s.Phase == "closed" || s.Phase == "closing" {
		return fmt.Errorf("Change worktrees are not available for setup")
	}
	cfg, err := c.lifecycleConfig()
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, name := range spec.Repos {
		selected[name] = true
	}
	if explicit {
		if !selected[names[0]] {
			return fmt.Errorf("repo %s is not selected by change.yaml", names[0])
		}
	} else {
		names = append([]string(nil), spec.Repos...)
		sort.Strings(names)
		for _, name := range names {
			if s.Repos[name].Setup == "running" {
				return fmt.Errorf("repo %s setup was interrupted; run aiw change setup %s to retry explicitly", name, name)
			}
		}
	}
	for _, name := range names {
		if !explicit && s.Repos[name].Setup == "success" {
			continue
		}
		if err := c.setupRepo(&s, name, cfg); err != nil {
			s.Phase = "setup-pending"
			_ = saveState(c.Home, s)
			return err
		}
	}
	if selectionReady(spec.Repos, s) {
		s.Phase = "ready"
	} else {
		s.Phase = "setup-pending"
	}
	return saveState(c.Home, s)
}

func selectedSetupsReady(names []string, s ChangeState) bool {
	for _, name := range names {
		entry := s.Repos[name]
		if entry.Path == "" || entry.Setup != "success" {
			return false
		}
	}
	return true
}

func selectionReady(names []string, s ChangeState) bool {
	if !selectedSetupsReady(names, s) {
		return false
	}
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	for name, entry := range s.Repos {
		if entry.Path != "" && !selected[name] {
			return false
		}
	}
	return true
}

func (c ChangeContext) cleanup(name string) error {
	s, err := c.state()
	if err != nil {
		return err
	}
	entry, ok := s.Repos[name]
	if !ok || entry.Path == "" {
		return fmt.Errorf("repo %s has no materialized worktree to clean", name)
	}
	if s.Phase != "closing" && entry.Cleanup != "failed" && entry.Cleanup != "running" {
		spec, err := loadSpec(c.Root, c.Config)
		if err != nil {
			return err
		}
		for _, selected := range spec.Repos {
			if selected == name {
				return fmt.Errorf("repo %s is still selected; cleanup is available after removal from change.yaml", name)
			}
		}
		if s.Phase == "ready" {
			s.Phase = "materializing"
			if err := saveState(c.Home, s); err != nil {
				return err
			}
		}
	}
	cfg, err := c.lifecycleConfig()
	if err != nil {
		return err
	}
	return c.cleanupRepo(&s, name, cfg, true)
}
