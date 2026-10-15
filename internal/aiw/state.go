package aiw

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/KiritoKing/aiw-cli/internal/localfs"
)

type RepoState struct {
	Path    string `json:"path"`
	Base    string `json:"base"`
	Store   string `json:"store"`
	Setup   string `json:"setup,omitempty"`
	Cleanup string `json:"cleanup,omitempty"`
}

type ChangeState struct {
	Project string               `json:"project"`
	Root    string               `json:"root"`
	Branch  string               `json:"branch"`
	Phase   string               `json:"phase"`
	Repos   map[string]RepoState `json:"repos"`
}

func statePath(home, project, change string) string {
	return filepath.Join(home, "changes", hash(project), hash(change)+".json")
}

func loadState(home, project, change string) (ChangeState, error) {
	var s ChangeState
	data, err := os.ReadFile(statePath(home, project, change))
	if os.IsNotExist(err) {
		s.Repos = map[string]RepoState{}
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Repos == nil {
		s.Repos = map[string]RepoState{}
	}
	return s, nil
}

func saveState(home string, s ChangeState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return localfs.AtomicWrite(statePath(home, s.Project, s.Branch), append(data, '\n'))
}
