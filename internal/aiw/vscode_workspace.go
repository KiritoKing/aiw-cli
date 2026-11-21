package aiw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const vscodeWorkspaceFile = "aiw.code-workspace"

type vscodeWorkspace struct {
	Folders []vscodeWorkspaceFolder `json:"folders"`
}

type vscodeWorkspaceFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (r commandEnv) syncVSCodeWorkspace(root string, cfg ProjectConfig) error {
	workspacePath := filepath.Join(root, vscodeWorkspaceFile)
	if err := assertVSCodeWorkspaceTarget(workspacePath); err != nil {
		return err
	}

	workspace := vscodeWorkspace{
		Folders: make([]vscodeWorkspaceFolder, 0, len(cfg.Repos)+1),
	}
	workspace.Folders = append(workspace.Folders, vscodeWorkspaceFolder{Name: cfg.Name, Path: "."})
	repoNames := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		repoNames = append(repoNames, name)
	}
	sort.Strings(repoNames)
	for _, name := range repoNames {
		workspace.Folders = append(workspace.Folders, vscodeWorkspaceFolder{
			Name: name,
			Path: filepath.Join("repos", name),
		})
	}

	data, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("workspace %s: encode JSON: %w", workspacePath, err)
	}
	data = append(data, '\n')

	if err := r.syncVSCodeWorkspaceExclude(root); err != nil {
		return err
	}
	if err := atomicWrite(workspacePath, data); err != nil {
		return fmt.Errorf("workspace %s: write: %w", workspacePath, err)
	}
	return nil
}

func (r commandEnv) autoSyncVSCodeWorkspace(root string, cfg ProjectConfig) error {
	if cfg.VSCode.AutoSync != nil && !*cfg.VSCode.AutoSync {
		return nil
	}
	return r.syncVSCodeWorkspace(root, cfg)
}

func assertVSCodeWorkspaceTarget(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("workspace %s: inspect target: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("workspace %s: refusing to replace non-regular file", path)
	}
	return nil
}

func (r commandEnv) syncVSCodeWorkspaceExclude(root string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("exclude for workspace %s: resolve root: %w", root, err)
	}
	excludePath, err := r.git(rootAbs, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return fmt.Errorf("exclude for workspace %s: resolve Git info/exclude: %w", rootAbs, err)
	}
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(rootAbs, excludePath)
	}

	err = r.withLock(excludePath+".aiw.lock", func() error {
		info, statErr := os.Lstat(excludePath)
		if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("exclude %s: inspect: %w", excludePath, statErr)
		}
		if statErr == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("exclude %s: refusing to replace non-regular file", excludePath)
		}

		contents, readErr := os.ReadFile(excludePath)
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("exclude %s: read: %w", excludePath, readErr)
		}
		if hasVSCodeExcludeRule(contents) {
			return nil
		}

		updated := append([]byte(nil), contents...)
		if len(updated) > 0 && updated[len(updated)-1] != '\n' {
			updated = append(updated, '\n')
		}
		updated = append(updated, []byte("/"+vscodeWorkspaceFile+"\n")...)
		if err := atomicWrite(excludePath, updated); err != nil {
			return fmt.Errorf("exclude %s: write: %w", excludePath, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("exclude %s: lock or sync: %w", excludePath, err)
	}
	return nil
}

func hasVSCodeExcludeRule(contents []byte) bool {
	for _, line := range bytes.Split(contents, []byte{'\n'}) {
		if strings.TrimSuffix(string(line), "\r") == "/"+vscodeWorkspaceFile {
			return true
		}
	}
	return false
}
