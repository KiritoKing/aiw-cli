package aiw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type RepoConfig struct {
	Remote  string       `yaml:"remote" json:"remote"`
	Base    string       `yaml:"base" json:"base"`
	Setup   *RepoCommand `yaml:"setup,omitempty" json:"setup,omitempty"`
	Cleanup *RepoCommand `yaml:"cleanup,omitempty" json:"cleanup,omitempty"`
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

type ChangeSpec struct {
	Version int      `yaml:"version"`
	Repos   []string `yaml:"repos"`
}

type RegistryEntry struct {
	Remote string `json:"remote"`
	Source string `json:"source"`
	Store  string `json:"store"`
}

type Registry struct {
	Repos map[string]RegistryEntry `json:"repos"`
}

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

var safeKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func dataHome() (string, error) {
	if custom := os.Getenv("AIW_HOME"); custom != "" {
		return filepath.Abs(custom)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "aiw"), nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

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
	if err := assertBranchName(cfg.Base); err != nil {
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
		if err := assertBranchName(repo.Base); err != nil {
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
	return atomicWrite(filepath.Join(root, "aiw.yaml"), data)
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
	return atomicWrite(specPath(root), data)
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".aiw-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func canonicalRemote(value string) string {
	value = strings.TrimSpace(value)
	if u, err := url.Parse(value); err == nil {
		if u.Scheme == "file" {
			return "file:" + filepath.Clean(u.Path)
		}
		if u.Host != "" {
			host := strings.ToLower(u.Hostname())
			if u.Port() != "" {
				host += ":" + u.Port()
			}
			return host + "/" + strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
		}
	}
	if at := strings.IndexByte(value, '@'); at >= 0 {
		if colon := strings.IndexByte(value[at:], ':'); colon >= 0 {
			return strings.ToLower(value[at+1:at+colon]) + "/" + strings.TrimSuffix(value[at+colon+1:], ".git")
		}
	}
	if abs, err := filepath.Abs(value); err == nil {
		return "file:" + filepath.Clean(abs)
	}
	return value
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
	return atomicWrite(registryPath(home), append(data, '\n'))
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
	return atomicWrite(statePath(home, s.Project, s.Branch), append(data, '\n'))
}
