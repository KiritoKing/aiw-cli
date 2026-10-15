package aiw

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KiritoKing/aiw-cli/internal/gitcli"
	"github.com/KiritoKing/aiw-cli/internal/localfs"
	"gopkg.in/yaml.v3"
)

type RepoConfig struct {
	Remote   string         `yaml:"remote" json:"remote"`
	Base     string         `yaml:"base" json:"base"`
	Setup    *RepoCommand   `yaml:"setup,omitempty" json:"setup,omitempty"`
	Cleanup  *RepoCommand   `yaml:"cleanup,omitempty" json:"cleanup,omitempty"`
	Metadata map[string]any `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type RepoCommand struct {
	Command string   `yaml:"command" json:"command"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty"`
}

type ProjectConfig struct {
	Version int                   `yaml:"version"`
	Name    string                `yaml:"name"`
	Base    string                `yaml:"base"`
	Repos   map[string]RepoConfig `yaml:"repos"`
}

var safeKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func projectRoot(cwd string) (string, error) {
	current, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "aiw.yaml")); err == nil {
			return filepath.EvalSymlinks(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("aiw.yaml not found above %s", cwd)
		}
		current = parent
	}
}

func loadProject(root string) (ProjectConfig, error) {
	data, err := os.ReadFile(filepath.Join(root, "aiw.yaml"))
	if err != nil {
		return ProjectConfig{}, err
	}
	return parseProject(data)
}

func parseProject(data []byte) (ProjectConfig, error) {
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("aiw.yaml: %w", err)
	}
	if cfg.Version != 1 || cfg.Name == "" || cfg.Base == "" {
		return cfg, fmt.Errorf("aiw.yaml needs version: 1, name, and base")
	}
	if err := gitcli.AssertBranchName(cfg.Base); err != nil {
		return cfg, err
	}
	for name, repo := range cfg.Repos {
		if !safeKey.MatchString(name) || repo.Remote == "" || repo.Base == "" {
			return cfg, fmt.Errorf("invalid repo %q in aiw.yaml", name)
		}
		if u, err := url.Parse(repo.Remote); err == nil && u.Scheme != "" {
			password := false
			if u.User != nil {
				_, password = u.User.Password()
			}
			if password || u.RawQuery != "" || u.Fragment != "" {
				return cfg, fmt.Errorf("repo %s remote must not embed credentials or URL parameters", name)
			}
		}
		if err := gitcli.AssertBranchName(repo.Base); err != nil {
			return cfg, fmt.Errorf("repo %s: %w", name, err)
		}
		for action, command := range map[string]*RepoCommand{"setup": repo.Setup, "cleanup": repo.Cleanup} {
			if command != nil && strings.TrimSpace(command.Command) == "" {
				return cfg, fmt.Errorf("repo %s %s command is empty", name, action)
			}
		}
	}
	return cfg, nil
}

func saveProject(root string, cfg ProjectConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return localfs.AtomicWrite(filepath.Join(root, "aiw.yaml"), data)
}
