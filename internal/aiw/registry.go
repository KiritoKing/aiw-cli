package aiw

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/KiritoKing/aiw-cli/internal/localfs"
)

type RegistryEntry struct {
	Remote string `json:"remote"`
	Source string `json:"source"`
	Store  string `json:"store"`
}

type Registry struct {
	Repos map[string]RegistryEntry `json:"repos"`
}

func registryPath(home string) string { return filepath.Join(home, "registry.json") }

func loadRegistry(home string) (Registry, error) {
	var r Registry
	r.Repos = map[string]RegistryEntry{}
	data, err := os.ReadFile(registryPath(home))
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	if r.Repos == nil {
		r.Repos = map[string]RegistryEntry{}
	}
	return r, nil
}

func saveRegistry(home string, r Registry) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return localfs.AtomicWrite(registryPath(home), append(data, '\n'))
}
