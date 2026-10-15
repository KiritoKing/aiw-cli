package commands

import (
	"fmt"

	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"github.com/spf13/cobra"
)

func newRepoCommand() *cobra.Command {
	service := aiw.Service{}
	cmd := newCommandGroup("repo", "Register, inspect, and explicitly synchronize business repositories")
	var jsonOutput bool
	list := &cobra.Command{
		Use: "list", Short: "List configured repositories and their local registration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.ListRepos(jsonOutput) },
	}
	list.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.AddCommand(list)

	var source string
	register := &cobra.Command{
		Use: "register <name>", Short: "Register an existing local checkout",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
			if source == "" {
				return fmt.Errorf("--source is required and must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error { return service.RegisterRepo(args[0], source) },
	}
	register.Flags().StringVar(&source, "source", "", "existing local checkout")
	_ = register.MarkFlagRequired("source")
	_ = register.MarkFlagDirname("source")
	cmd.AddCommand(register)

	cmd.AddCommand(&cobra.Command{
		Use: "scan <dir>", Short: "Find matching local checkouts under a directory", Args: cobra.ExactArgs(1),
		RunE:              func(cmd *cobra.Command, args []string) error { return service.ScanRepos(args[0]) },
		ValidArgsFunction: cobra.FixedCompletions(nil, cobra.ShellCompDirectiveFilterDirs),
	})
	cmd.AddCommand(&cobra.Command{
		Use: "sync [name]", Short: "Explicitly fetch one or all registered repositories", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return service.SyncRepos(name)
		},
	})
	return cmd
}
