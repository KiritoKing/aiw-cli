package aiw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Historical refs are used only to recover definitions for deleted repos in older state.
func (c ChangeContext) historicalRepo(name string) (RepoConfig, bool) {
	for _, ref := range []string{"HEAD", "refs/remotes/origin/" + c.Config.Base, "refs/heads/" + c.Config.Base} {
		data, err := c.git(c.Root, "show", ref+":aiw.yaml")
		if err != nil {
			continue
		}
		// Older manifests could contain fields accepted before strict decoding.
		var cfg ProjectConfig
		if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
			continue
		}
		if repo, ok := cfg.Repos[name]; ok {
			if repo.Cleanup != nil && strings.TrimSpace(repo.Cleanup.Command) == "" {
				repo.Cleanup = nil
			}
			return repo, true
		}
	}
	return RepoConfig{}, false
}

func (c ChangeContext) cleanupCommand(name string, entry RepoState) *RepoCommand {
	if repo, ok := c.Config.Repos[name]; ok {
		return repo.Cleanup
	}
	if entry.CleanupRecorded || entry.Cleanup != nil {
		return entry.Cleanup
	}
	repo, _ := c.historicalRepo(name)
	return repo.Cleanup
}

func (c ChangeContext) stateStore(name string, entry RepoState) (string, error) {
	if entry.Store != "" {
		return entry.Store, nil
	}
	registry, err := loadRegistry(c.Home)
	if err != nil {
		return "", err
	}
	if _, ok := c.Config.Repos[name]; ok {
		return c.storeFor(c.Config, registry, name)
	}
	repo, ok := c.historicalRepo(name)
	if !ok {
		return "", fmt.Errorf("repo %s has no recorded store or historical remote", name)
	}
	return c.registeredStore(registry, name, repo)
}

func (c ChangeContext) executeRepoCommand(name, path, action string, command *RepoCommand) error {
	if command == nil {
		return nil
	}
	bin := command.Command
	if strings.ContainsRune(bin, os.PathSeparator) && !filepath.IsAbs(bin) {
		bin = filepath.Join(path, bin)
	}
	cmd := exec.CommandContext(c.ctx, bin, command.Args...)
	cmd.Dir = path
	cmd.Env = append(os.Environ(),
		"AIW_LIFECYCLE_ACTIVE=1",
		"AIW_LIFECYCLE_ACTION="+action,
		"AIW_CHANGE_ROOT="+c.Root,
		"AIW_CHANGE_BRANCH="+c.Branch,
		"AIW_REPO_NAME="+name,
		"AIW_REPO_PATH="+path,
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.In, c.Out, c.ErrOut
	fmt.Fprintf(c.Out, "%s %s in %s\n", action, name, path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repo %s %s failed: %w", name, action, err)
	}
	return nil
}

func (c ChangeContext) setupRepo(s *ChangeState, name string) error {
	entry := s.Repos[name]
	if entry.Path == "" {
		return fmt.Errorf("repo %s has no materialized worktree", name)
	}
	repo := c.Config.Repos[name]
	if entry.Store == "" {
		registry, err := loadRegistry(c.Home)
		if err != nil {
			return err
		}
		entry.Store, err = c.storeFor(c.Config, registry, name)
		if err != nil {
			return err
		}
	}
	if entry.Setup == "running" {
		fmt.Fprintf(c.ErrOut, "repo %s setup was interrupted; retry may repeat script effects\n", name)
	}
	if err := c.checkOwned(name, entry.Path, entry.Store); err != nil {
		return err
	}
	entry.Setup = "running"
	entry.recordCleanup(repo.Cleanup)
	s.Repos[name] = entry
	if err := saveState(c.Home, *s); err != nil {
		return err
	}
	err := c.executeRepoCommand(name, entry.Path, "setup", repo.Setup)
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}
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

func (c ChangeContext) cleanupRepo(s *ChangeState, name string, retry bool) error {
	entry := s.Repos[name]
	if entry.Path == "" {
		return fmt.Errorf("repo %s has no materialized worktree", name)
	}
	if entry.CleanupStatus == "success" {
		return nil
	}
	if (entry.CleanupStatus == "failed" || entry.CleanupStatus == "running") && !retry {
		return fmt.Errorf("repo %s cleanup is %s; run aiw change cleanup %s before retrying release", name, entry.CleanupStatus, name)
	}
	if entry.Store == "" {
		var err error
		entry.Store, err = c.stateStore(name, entry)
		if err != nil {
			return err
		}
	}
	if entry.CleanupStatus == "running" {
		fmt.Fprintf(c.ErrOut, "repo %s cleanup was interrupted; retry may repeat script effects\n", name)
	}
	if err := c.checkOwned(name, entry.Path, entry.Store); err != nil {
		return err
	}
	command := c.cleanupCommand(name, entry)
	entry.recordCleanup(command)
	entry.CleanupStatus = "running"
	s.Repos[name] = entry
	if err := saveState(c.Home, *s); err != nil {
		return err
	}
	err := c.executeRepoCommand(name, entry.Path, "cleanup", command)
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}
	if err != nil {
		entry.CleanupStatus = "failed"
	} else {
		entry.CleanupStatus = "success"
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
		if err := c.setupRepo(&s, name); err != nil {
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
	if s.Phase != "closing" && entry.CleanupStatus != "failed" && entry.CleanupStatus != "running" {
		if _, declared := c.Config.Repos[name]; declared {
			spec, err := readSpec(c.Root)
			if err != nil {
				return err
			}
			for _, selected := range spec.Repos {
				if selected == name {
					return fmt.Errorf("repo %s is still selected; cleanup is available after removal from change.yaml", name)
				}
			}
		}
		if s.Phase == "ready" {
			s.Phase = "materializing"
			if err := saveState(c.Home, s); err != nil {
				return err
			}
		}
	}
	return c.cleanupRepo(&s, name, true)
}
