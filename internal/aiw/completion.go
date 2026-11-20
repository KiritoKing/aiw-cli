package aiw

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

func completeDirectory(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveFilterDirs
}

func completeRepos(runEnv func(*cobra.Command) (commandEnv, error)) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		var matches []string
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		r, err := runEnv(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		root, err := projectRoot(r.Cwd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := r.loadProject(root)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		selected, _ := cmd.Flags().GetStringArray("repo")
		for _, name := range repoNames(cfg) {
			if !strings.HasPrefix(name, prefix) || slices.Contains(selected, name) {
				continue
			}
			repo := cfg.Repos[name]
			detail := "base " + repo.Base
			if description := repoDescription(repo.Metadata); description != "" {
				detail += " · " + terminalLabel(description)
			}
			matches = append(matches, cobra.CompletionWithDesc(name, detail))
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeBranches(runEnv func(*cobra.Command) (commandEnv, error)) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		r, err := runEnv(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		home, project, err := r.changeIndex()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		states, err := listChangeStates(home, project)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var matches []string
		for _, state := range states {
			if strings.HasPrefix(state.Branch, prefix) {
				matches = append(matches, cobra.CompletionWithDesc(state.Branch, state.Phase))
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

func repoDescription(metadata map[string]any) string {
	description, _ := metadata["description"].(string)
	return strings.Join(strings.Fields(description), " ")
}
