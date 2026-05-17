package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/output"
	"github.com/ivan/infra-shelf/internal/shelfcore"
)

func NewRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <app>",
		Short: "Restore app data from backup",
		Args:  cobra.ExactArgs(1),
		RunE:  runRestore,
	}
	cmd.Flags().StringP("services", "s", "", "limit to specific services")
	cmd.Flags().String("file", "", "explicit backup file path (defaults to latest)")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")
	return cmd
}

func runRestore(cmd *cobra.Command, args []string) error {
	appName := args[0]
	filePath, _ := cmd.Flags().GetString("file")
	force, _ := cmd.Flags().GetBool("force")

	engine, _, err := buildEngine()
	if err != nil {
		return err
	}

	if filePath != "" {
		if !force {
			prompt := fmt.Sprintf("Restore %q from %s? [y/N] ", appName, filepath.Base(filePath))
			if !confirm(prompt) {
				output.Info("Cancelled.")
				return nil
			}
		}
		return engine.RestoreFromFile(cmd.Context(), appName, filePath)
	}

	services, err := parseOptionalServicesFlag(cmd)
	if err != nil {
		return err
	}
	opts := shelfcore.RestoreOptions{Services: services}

	plan, err := engine.PlanRestore(cmd.Context(), appName, opts)
	if err != nil {
		return err
	}
	if !force {
		fmt.Println()
		output.Title(fmt.Sprintf("Restore plan for %q:", appName))
		for _, p := range plan {
			fmt.Printf("  %s <- %s\n", p.Service, filepath.Base(p.File))
		}
		fmt.Println()
		if !confirm("Proceed with restore? [y/N] ") {
			output.Info("Cancelled.")
			return nil
		}
	}

	return engine.RestoreApp(cmd.Context(), appName, opts)
}
