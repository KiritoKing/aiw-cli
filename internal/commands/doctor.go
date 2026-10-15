package commands

import (
	"github.com/KiritoKing/aiw-cli/internal/aiw"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	service := aiw.Service{}
	return &cobra.Command{
		Use: "doctor", Short: "Check Git and the current Agent Repo", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return service.Doctor() },
	}
}
