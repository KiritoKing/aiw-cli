package aiw

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KiritoKing/aiw-cli/internal/localfs"
	"gopkg.in/yaml.v3"
)

type ChangeSpec struct {
	Version int      `yaml:"version"`
	Repos   []string `yaml:"repos"`
}

func specPath(root string) string { return filepath.Join(root, ".aiw", "change.yaml") }

func loadSpec(root string, cfg ProjectConfig) (ChangeSpec, error) {
	data, err := os.ReadFile(specPath(root))
	if err != nil {
		return ChangeSpec{}, fmt.Errorf("change selection missing: %s; define repos during Propose, then run materialize", specPath(root))
	}
	var spec ChangeSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return spec, err
	}
	if spec.Version != 1 || len(spec.Repos) == 0 {
		return spec, fmt.Errorf("change.yaml needs version: 1 and at least one repo")
	}
	seen := map[string]bool{}
	for _, name := range spec.Repos {
		if _, ok := cfg.Repos[name]; !ok {
			return spec, fmt.Errorf("unknown repo %q in change.yaml", name)
		}
		if seen[name] {
			return spec, fmt.Errorf("duplicate repo %q in change.yaml", name)
		}
		seen[name] = true
	}
	return spec, nil
}

func saveSpec(root string, spec ChangeSpec) error {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(specPath(root)), 0755); err != nil {
		return err
	}
	return localfs.AtomicWrite(specPath(root), data)
}
