package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/output"
)

func NewCredentialsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "credentials <app>",
		Short: "Print .env block for an app",
		Args:  cobra.ExactArgs(1),
		RunE:  runCredentials,
	}
}

func runCredentials(cmd *cobra.Command, args []string) error {
	engine, _, err := buildEngine()
	if err != nil {
		return err
	}
	body, err := engine.Credentials(args[0])
	if err != nil {
		return err
	}
	if body == "" {
		output.Warn(fmt.Sprintf("App %q has no services attached.", args[0]))
		return nil
	}
	fmt.Println(body)
	return nil
}
