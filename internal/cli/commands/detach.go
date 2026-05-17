package commands

import (
	"github.com/spf13/cobra"
)

func NewDetachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detach <app>",
		Short: "Detach an addon from an app (registry-only)",
		Args:  cobra.ExactArgs(1),
		RunE:  runDetach,
	}
	cmd.Flags().StringP("services", "s", "", "comma-separated services (signoz)")
	return cmd
}

func runDetach(cmd *cobra.Command, args []string) error {
	services, err := parseServicesFlag(cmd)
	if err != nil {
		return err
	}
	if err := requireAtLeastOneService(services, "signoz"); err != nil {
		return err
	}

	engine, _, err := buildEngine()
	if err != nil {
		return err
	}
	return engine.DetachServices(cmd.Context(), args[0], services)
}
