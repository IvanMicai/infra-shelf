package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/output"
)

func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "start [docker compose args...]",
		Short:              "Start infrastructure (docker compose up -d)",
		DisableFlagParsing: true,
		RunE:               runStart,
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	full := append([]string{"compose", "--env-file", ".env", "up", "-d"}, args...)
	output.Info(fmt.Sprintf("docker %v", full))

	c := exec.CommandContext(cmd.Context(), "docker", full...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("docker compose failed: %w", err)
	}
	output.Success("infrastructure started")
	return nil
}
