package aiw

import (
	"encoding/json"
	"sort"
)

type RepoListRow struct {
	Name       string         `json:"name"`
	Remote     string         `json:"remote"`
	Base       string         `json:"base"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Registered bool           `json:"registered"`
	Source     string         `json:"source,omitempty"`
	Store      string         `json:"store,omitempty"`
}

func (r commandEnv) repoList(root, home string) error {
	cfg, err := r.loadProject(root)
	if err != nil {
		return err
	}
	registry, err := loadRegistry(home)
	if err != nil {
		return err
	}
	rows := make([]RepoListRow, 0, len(cfg.Repos))
	for _, name := range repoNames(cfg) {
		repo := cfg.Repos[name]
		row := RepoListRow{Name: name, Remote: redactCredentials(repo.Remote), Base: repo.Base, Metadata: repo.Metadata}
		if entry, ok := registry.Repos[canonicalRemote(repo.Remote)]; ok {
			row.Registered, row.Source, row.Store = true, entry.Source, entry.Store
		}
		rows = append(rows, row)
	}
	if r.jsonOutput {
		return r.writeJSON(rows)
	}
	cells := make([][]string, 0, len(rows))
	for _, row := range rows {
		status, description := "unregistered", repoDescription(row.Metadata)
		if row.Registered {
			status = "registered"
			if description != "" {
				description = row.Store + " · " + description
			} else {
				description = row.Store
			}
		}
		cells = append(cells, []string{row.Name, row.Base, status, description})
	}
	return r.printTable([]string{"REPO", "BASE", "STATUS", "STORE / DESCRIPTION"}, cells, 2)
}

func repoNames(cfg ProjectConfig) []string {
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r commandEnv) writeJSON(value any) error {
	encoder := json.NewEncoder(r.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
