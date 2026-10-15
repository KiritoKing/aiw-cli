package commands

import (
	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"github.com/spf13/cobra"
)

func newAgentsCommand() *cobra.Command {
	service := aiw.Service{}
	cmd := newCommandGroup("agents", "Maintain AIW-managed Agent Repo instructions")
	cmd.AddCommand(&cobra.Command{
		Use: "sync", Short: "Update the AIW block in AGENTS.md", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.SyncAgents() },
	})
	return cmd
}
