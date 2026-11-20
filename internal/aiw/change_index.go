package aiw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (r commandEnv) changeIndex() (home, project string, err error) {
	root, err := projectRoot(r.Cwd)
	if err != nil {
		return "", "", err
	}
	project, err = r.gitCommonDir(root)
	if err != nil {
		return "", "", err
	}
	home, err = r.dataHome()
	return home, project, err
}

func listChangeStates(home, project string) ([]ChangeState, error) {
	var states []ChangeState
	dir := filepath.Join(home, "changes", hash(project))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return states, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var state ChangeState
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("read Change state %s: %w", path, err)
		}
		if state.Project != project || state.Branch == "" || path != statePath(home, project, state.Branch) {
			return nil, fmt.Errorf("Change state identity mismatch: %s", path)
		}
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Branch < states[j].Branch })
	return states, nil
}

type ChangeListRow struct {
	Branch     string `json:"branch"`
	Phase      string `json:"phase"`
	Root       string `json:"root"`
	RootExists bool   `json:"rootExists"`
}

func rootExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func (r commandEnv) changeList() error {
	home, project, err := r.changeIndex()
	if err != nil {
		return err
	}
	states, err := listChangeStates(home, project)
	if err != nil {
		return err
	}
	rows := make([]ChangeListRow, 0, len(states))
	for _, state := range states {
		exists, err := rootExists(state.Root)
		if err != nil {
			return err
		}
		rows = append(rows, ChangeListRow{Branch: state.Branch, Phase: state.Phase, Root: state.Root, RootExists: exists})
	}
	if r.jsonOutput {
		return r.writeJSON(rows)
	}
	cells := make([][]string, 0, len(rows))
	for _, row := range rows {
		status := "available"
		if !row.RootExists {
			status = "stale"
		}
		cells = append(cells, []string{row.Branch, row.Phase, status, row.Root})
	}
	return r.printTable([]string{"BRANCH", "PHASE", "STATUS", "ROOT"}, cells, 2)
}

func (r commandEnv) changePath(branch string) error {
	home, project, err := r.changeIndex()
	if err != nil {
		return err
	}
	state, err := loadState(home, project, branch)
	if err != nil {
		return err
	}
	if state.Root == "" {
		return fmt.Errorf("Change %s not registered on this machine", branch)
	}
	if state.Project != project || state.Branch != branch {
		return fmt.Errorf("Change state identity mismatch")
	}
	if info, err := os.Stat(state.Root); err != nil || !info.IsDir() {
		return fmt.Errorf("control-plane worktree unavailable: %s", state.Root)
	}
	fmt.Fprintln(r.Out, state.Root)
	return nil
}
