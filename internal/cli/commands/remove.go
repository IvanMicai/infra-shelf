package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/output"
)

func NewRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <app>",
		Short: "Remove resources for an app",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}
	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")
	return cmd
}

func runRemove(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	appName := args[0]

	if !force {
		if !confirm(fmt.Sprintf("Remove all resources for %q? [y/N] ", appName)) {
			output.Info("Cancelled.")
			return nil
		}
	}

	engine, _, err := buildEngine()
	if err != nil {
		return err
	}
	return engine.RemoveApp(cmd.Context(), appName)
}

// confirm reads a single line from stdin and returns true iff the user typed
// "y" or "Y" (matching the legacy TS readline prompt behavior).
func confirm(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "y")
}
