package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/envspec"
	"github.com/ivan/infra-shelf/internal/output"
	"github.com/ivan/infra-shelf/internal/registry"
	"github.com/ivan/infra-shelf/internal/shelfcore"
)

func NewSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup <app>",
		Short: "Provision resources for an app",
		Args:  cobra.ExactArgs(1),
		RunE:  runSetup,
	}
	cmd.Flags().StringP("services", "s", "", "comma-separated services (postgres,redis,rabbitmq,aistor,signoz)")
	cmd.Flags().String("env", "", "tag a single app with this environment")
	cmd.Flags().String("envs", "", "expand into sibling apps (CSV of envs)")
	cmd.Flags().Bool("full-access", false, "grant full data-plane access (no key/db isolation)")
	return cmd
}

func runSetup(cmd *cobra.Command, args []string) error {
	appName := args[0]
	services, err := parseServicesFlag(cmd)
	if err != nil {
		return err
	}
	if err := requireAtLeastOneService(services, "postgres,redis,rabbitmq,aistor"); err != nil {
		return err
	}

	envsRaw, _ := cmd.Flags().GetString("envs")
	envRaw, _ := cmd.Flags().GetString("env")
	envs, err := envspec.ParseEnvs(envsRaw)
	if err != nil {
		return err
	}
	env, err := envspec.ParseSingleEnv(envRaw)
	if err != nil {
		return err
	}
	if envs != nil && env != "" {
		return errors.New("--envs and --env are mutually exclusive (use --envs to expand into siblings, --env to tag a single app)")
	}
	fullAccess, _ := cmd.Flags().GetBool("full-access")

	engine, _, err := buildEngine()
	if err != nil {
		return err
	}

	results, err := engine.SetupApp(cmd.Context(), appName, shelfcore.SetupOptions{
		Services:   services,
		Envs:       envs,
		Env:        env,
		FullAccess: fullAccess,
	})
	if err != nil {
		return err
	}

	for _, result := range results {
		fmt.Println()
		output.Title(fmt.Sprintf("App %q ready!", result.Name))
		printEnvBlocks(result.Entry)
		fmt.Println()
	}
	return nil
}

// printEnvBlocks dumps the .env block for an app entry to stdout. Used by
// setup/add to echo what was just provisioned.
func printEnvBlocks(entry registry.AppEntry) {
	app := registry.App{Name: "", Entry: entry}
	body := app.EnvFile()
	if body != "" {
		fmt.Println(body)
	}
}
