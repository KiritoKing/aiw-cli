package commands

import (
	"fmt"
	"os"

	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"github.com/spf13/cobra"
)

// Run builds a fresh command tree so flags cannot leak between invocations.
// main owns error printing and the process exit code.
func Run(args []string) error {
	cmd := newRootCommand()
	// Cobra treats nil as os.Args[1:]; Run(nil) means an empty invocation.
	if args == nil {
		args = []string{}
	}
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newRootCommand() *cobra.Command {
	service := aiw.Service{}
	cmd := newCommandGroup("aiw", "Assemble business Git worktrees inside an Agent Repo worktree")
	cmd.Long = `AIW assembles business Git worktrees inside an Agent Repo worktree created by your control plane.

During Propose, write .aiw/change.yaml. Before dispatching business tasks, run
aiw change materialize; it runs selected repository setup scripts. During cleanup,
the control plane calls aiw change close before removing the root worktree.`
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.AddCommand(newInitCommand(), newRepoCommand(), newChangeCommand(), newCompletionCommand())
	cmd.AddCommand(&cobra.Command{
		Use: "version", Short: "Print the AIW version", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), service.Version())
			return err
		},
	})
	cmd.AddCommand(newDoctorCommand(), newAgentsCommand())
	return cmd
}

func newCommandGroup(use, short string) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
}
