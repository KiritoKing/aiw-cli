package commands

import (
	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := newCommandGroup("completion", "Generate shell completion from the AIW command tree")
	for _, shell := range []string{"bash", "zsh"} {
		shellCmd := &cobra.Command{
			Use: shell, Short: "Generate " + shell + " completion", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if shell == "bash" {
					return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
				}
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			},
		}
		if shell == "bash" {
			shellCmd.Long = "Generate bash completion. To use it, install and load bash-completion first\n(the shell function _get_comp_words_by_ref must be available), then source the generated script."
		}
		cmd.AddCommand(shellCmd)
	}
	return cmd
}
