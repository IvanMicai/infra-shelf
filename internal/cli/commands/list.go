package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/IvanMicai/infra-shelf/internal/output"
	"github.com/IvanMicai/infra-shelf/internal/registry"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all provisioned apps",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
	cmd.Flags().BoolP("json", "j", false, "emit raw JSON registry")
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")

	engine, _, err := buildEngine()
	if err != nil {
		return err
	}
	apps, err := engine.ListApps()
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		output.Info("No apps provisioned yet.")
		return nil
	}

	if asJSON {
		// Match the legacy TS shape exactly: a name → AppEntry map.
		out := map[string]registry.AppEntry{}
		for _, a := range apps {
			out[a.Name] = a.Entry
		}
		payload, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}

	for _, app := range apps {
		output.Title(app.Name)
		created := "unknown"
		if t := app.CreatedAtTime(); !t.IsZero() {
			created = t.Format(time.DateOnly)
		}
		output.Dim(fmt.Sprintf("Created: %s | Services: %v", created, app.ServiceNames()))
		body := app.EnvFile()
		if body != "" {
			fmt.Println(body)
		}
		fmt.Println()
	}
	return nil
}
