package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/output"
	"github.com/ivan/infra-shelf/internal/shelfcore"
)

func NewBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup [<app>]",
		Short: "Backup app data",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runBackup,
	}
	cmd.Flags().StringP("services", "s", "", "limit to specific services")
	cmd.Flags().BoolP("all", "a", false, "backup all apps")

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <app> <file>",
		Short: "Delete a backup file",
		Args:  cobra.ExactArgs(2),
		RunE:  runBackupDelete,
	})
	return cmd
}

func runBackup(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	services, err := parseOptionalServicesFlag(cmd)
	if err != nil {
		return err
	}

	engine, cfg, err := buildEngine()
	if err != nil {
		return err
	}

	opts := shelfcore.BackupOptions{Services: services}

	if all {
		_, err := engine.BackupAll(cmd.Context(), opts)
		if err != nil {
			return err
		}
	} else {
		if len(args) == 0 {
			return fmt.Errorf("app name is required (use --all to backup all apps)")
		}
		output.Title(fmt.Sprintf("Backing up %q...", args[0]))
		if _, err := engine.BackupApp(cmd.Context(), args[0], opts); err != nil {
			return err
		}
	}

	fmt.Println()
	output.Success(fmt.Sprintf("Backups saved to %s/", cfg.BackupsDir))
	return nil
}

func runBackupDelete(cmd *cobra.Command, args []string) error {
	engine, _, err := buildEngine()
	if err != nil {
		return err
	}
	if err := engine.DeleteBackup(args[0], args[1]); err != nil {
		return err
	}
	output.Success(fmt.Sprintf("removed %s/%s", args[0], args[1]))
	return nil
}

// parseOptionalServicesFlag treats an empty -s as "all services" rather than
// "validation error" (matching backup/restore semantics in the TS CLI).
func parseOptionalServicesFlag(cmd *cobra.Command) ([]string, error) {
	raw, _ := cmd.Flags().GetString("services")
	if raw == "" {
		return nil, nil
	}
	return parseServicesFlag(cmd)
}
