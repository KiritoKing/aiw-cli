package aiw

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KiritoKing/aiw-cli/internal/localfs"
)

// Service exposes AIW operations without a dependency on the command framework.
// Its zero value uses the current directory and the existing AIW environment.
type Service struct{}

func (Service) Init(path string, skipSkills bool) error {
	if path == "" {
		return fmt.Errorf("agent-repo-path must not be empty")
	}
	return initializeAgentRepo(path, skipSkills)
}

func (Service) Version() string   { return buildVersion() }
func (Service) Doctor() error     { return runDoctor() }
func (Service) SyncAgents() error { return syncCurrentAgentInstructions() }

func (Service) ListRepos(jsonOutput bool) error {
	root, home, err := repoContext()
	if err != nil {
		return err
	}
	return listRepos(root, home, jsonOutput)
}

func (Service) RegisterRepo(name, source string) error {
	return mutateRepos(func(root, home string) error { return registerRepo(root, home, name, source) })
}

func (Service) ScanRepos(path string) error {
	return mutateRepos(func(root, home string) error { return scanRepos(root, home, path) })
}

func (Service) SyncRepos(name string) error {
	return mutateRepos(func(root, home string) error { return syncRepo(root, home, name) })
}

func repoContext() (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		return "", "", err
	}
	home, err := dataHome()
	return root, home, err
}

func mutateRepos(action func(root, home string) error) error {
	root, home, err := repoContext()
	if err != nil {
		return err
	}
	return localfs.WithLock(filepath.Join(home, "registry.lock"), func() error { return action(root, home) })
}

func (Service) SelectRepos(names []string) error {
	return withCurrentChange(func(c ChangeContext) error { return c.selectRepos(names) })
}

func (Service) UpdateSelection(action, name string) error {
	if action != "add" && action != "remove" {
		return fmt.Errorf("unknown selection action %q", action)
	}
	return withCurrentChange(func(c ChangeContext) error { return c.updateSelection(action, name) })
}

func (Service) Materialize() error {
	return withCurrentChange(func(c ChangeContext) error { return c.materialize(false) })
}

func (Service) Setup(names []string) error {
	return withCurrentChange(func(c ChangeContext) error { return c.setup(names) })
}

func (Service) Cleanup(name string) error {
	return withCurrentChange(func(c ChangeContext) error { return c.cleanup(name) })
}

func (Service) Repair() error {
	return withCurrentChange(func(c ChangeContext) error { return c.repair() })
}

func (Service) Status(jsonOutput bool) error {
	return withCurrentChange(func(c ChangeContext) error { return c.status(jsonOutput) })
}

func (Service) Close(options CloseOptions) error {
	return withCurrentChange(func(c ChangeContext) error { return c.close(options) })
}

func ensureChangeAllowed() error {
	if os.Getenv("AIW_LIFECYCLE_ACTIVE") == "1" {
		return fmt.Errorf("AIW Change commands cannot run inside a repository setup or cleanup script")
	}
	return nil
}

func withCurrentChange(action func(ChangeContext) error) error {
	if err := ensureChangeAllowed(); err != nil {
		return err
	}
	c, err := currentChange()
	if err != nil {
		return err
	}
	return changeLock(c, func() error { return action(c) })
}

func (Service) ListChanges() error {
	if err := ensureChangeAllowed(); err != nil {
		return err
	}
	return changeLookup("list", "")
}

func (Service) ChangePath(branch string) error {
	if err := ensureChangeAllowed(); err != nil {
		return err
	}
	return changeLookup("path", branch)
}
