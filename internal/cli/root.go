package cli

import (
	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/cli/commands"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "shelf",
		Short:         "infra-shelf CLI — provision and manage per-app credentials on the shared infrastructure",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		commands.NewSetupCmd(),
		commands.NewAddCmd(),
		commands.NewDetachCmd(),
		commands.NewListCmd(),
		commands.NewCredentialsCmd(),
		commands.NewRemoveCmd(),
		commands.NewBackupCmd(),
		commands.NewRestoreCmd(),
		commands.NewStartCmd(),
		commands.NewStatusCmd(),
		commands.NewScheduleCmd(),
		commands.NewRegistryCmd(),
	)

	return root
}
