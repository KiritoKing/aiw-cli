package commands

import (
	"fmt"

	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	service := aiw.Service{}
	var skipSkills bool
	cmd := &cobra.Command{
		Use: "init <agent-repo-path>", Short: "Initialize a fixed Agent Repo",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
			if args[0] == "" {
				return fmt.Errorf("agent-repo-path must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return service.Init(args[0], skipSkills)
		},
	}
	cmd.Flags().BoolVar(&skipSkills, "skip-skills", false, "skip installing AIW skills")
	cmd.ValidArgsFunction = cobra.FixedCompletions(nil, cobra.ShellCompDirectiveFilterDirs)
	return cmd
}
