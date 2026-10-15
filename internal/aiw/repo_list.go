package aiw

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KiritoKing/aiw-cli/internal/gitcli"
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

func listRepos(root, home string, jsonOutput bool) error {
	cfg, err := loadProject(root)
	if err != nil {
		return err
	}
	registry, err := loadRegistry(home)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	if jsonOutput {
		rows := make([]RepoListRow, 0, len(names))
		for _, name := range names {
			repo := cfg.Repos[name]
			row := RepoListRow{Name: name, Remote: gitcli.RedactCredentials(repo.Remote), Base: repo.Base, Metadata: repo.Metadata}
			if entry, ok := registry.Repos[canonicalRemote(repo.Remote)]; ok {
				row.Registered, row.Source, row.Store = true, entry.Source, entry.Store
			}
			rows = append(rows, row)
		}
		out, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	for _, name := range names {
		state := "unregistered"
		if entry, ok := registry.Repos[canonicalRemote(cfg.Repos[name].Remote)]; ok {
			state = entry.Store
		}
		fmt.Printf("%-16s %-16s %s\n", name, cfg.Repos[name].Base, state)
	}
	return nil
}
