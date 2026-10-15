package commands

import (
	"fmt"

	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"github.com/spf13/cobra"
)

func newChangeCommand() *cobra.Command {
	service := aiw.Service{}
	cmd := newCommandGroup("change", "Select, materialize, inspect, and release business worktrees")

	var names []string
	selectCmd := &cobra.Command{
		Use: "select", Short: "Write and commit this Change's repository selection",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if len(names) == 0 {
				return fmt.Errorf("at least one --repo is required")
			}
			for _, name := range names {
				if name == "" {
					return fmt.Errorf("--repo must not be empty")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error { return service.SelectRepos(names) },
	}
	// A repeated --repo is one value, matching the previous parser (no CSV splitting).
	selectCmd.Flags().StringArrayVar(&names, "repo", nil, "repository to select (repeat for multiple repositories)")
	_ = selectCmd.MarkFlagRequired("repo")
	cmd.AddCommand(selectCmd)
	for _, action := range []string{"add", "remove"} {
		cmd.AddCommand(&cobra.Command{
			Use: action + "-repo <name>", Short: action + " a repository in the selection", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error { return service.UpdateSelection(action, args[0]) },
		})
	}
	cmd.AddCommand(&cobra.Command{
		Use: "materialize", Short: "Reconcile selected worktrees and run repository setup", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.Materialize() },
	})
	cmd.AddCommand(&cobra.Command{
		Use: "setup [name]", Short: "Resume pending setup or rerun setup for one repository", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return service.Setup(args) },
	})
	cmd.AddCommand(&cobra.Command{
		Use: "cleanup <name>", Short: "Run cleanup for one repository", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return service.Cleanup(args[0]) },
	})
	cmd.AddCommand(&cobra.Command{
		Use: "repair", Short: "Repair worktrees and links without replaying lifecycle scripts", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.Repair() },
	})

	var jsonOutput bool
	status := &cobra.Command{
		Use: "status", Short: "Inspect the current Change and repository readiness", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.Status(jsonOutput) },
	}
	status.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.AddCommand(status)

	var options aiw.CloseOptions
	closeCmd := &cobra.Command{
		Use: "close", Short: "Run cleanup and release AIW-managed business worktrees", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.Close(options) },
	}
	closeCmd.Flags().BoolVar(&options.AllowUnpushed, "allow-unpushed", false, "close when remote branch is not confirmed equal")
	closeCmd.Flags().BoolVar(&options.Discard, "discard-changes", false, "discard local changes in AIW business worktrees")
	cmd.AddCommand(closeCmd)

	cmd.AddCommand(&cobra.Command{
		Use: "list", Short: "List locally registered Changes", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.ListChanges() },
	})
	for _, action := range []string{"path", "open"} {
		cmd.AddCommand(&cobra.Command{
			Use: action + " <branch>", Short: "Print an existing control-plane Change worktree path", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error { return service.ChangePath(args[0]) },
		})
	}
	return cmd
}
