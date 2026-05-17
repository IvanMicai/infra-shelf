package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/docker"
	"github.com/ivan/infra-shelf/internal/output"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show infrastructure container status",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, _ []string) error {
	output.Title("Infrastructure Status")
	for _, status := range docker.ListStatus(cmd.Context()) {
		icon, label := iconForState(status.State, status.Exists)
		fmt.Printf("  %-14s %s %s\n", status.Service, icon, label)
	}
	fmt.Println()
	return nil
}

func iconForState(state string, exists bool) (string, string) {
	switch {
	case !exists:
		return "⏹", "not created"
	case state == "running":
		return "🟢", "running"
	default:
		return "🔴", state
	}
}
