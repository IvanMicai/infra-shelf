package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/envspec"
	"github.com/ivan/infra-shelf/internal/output"
	"github.com/ivan/infra-shelf/internal/shelfcore"
)

func NewAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <app>",
		Short: "Attach more services to an existing app",
		Args:  cobra.ExactArgs(1),
		RunE:  runAdd,
	}
	cmd.Flags().StringP("services", "s", "", "comma-separated services")
	cmd.Flags().String("env", "", "single env to apply")
	cmd.Flags().String("envs", "", "siblings (CSV)")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	appName := args[0]
	services, err := parseServicesFlag(cmd)
	if err != nil {
		return err
	}
	if err := requireAtLeastOneService(services, "aistor,signoz,..."); err != nil {
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
		return errors.New("--envs and --env are mutually exclusive")
	}

	engine, _, err := buildEngine()
	if err != nil {
		return err
	}

	results, err := engine.AddServices(cmd.Context(), appName, shelfcore.AddOptions{
		Services: services, Envs: envs, Env: env,
	})
	if err != nil {
		return err
	}

	for _, result := range results {
		if len(result.Provisioned) == 0 {
			continue
		}
		fmt.Println()
		output.Title(fmt.Sprintf("Services attached to %q:", result.Name))
		printEnvBlocks(result.Entry)
		fmt.Println()
	}
	return nil
}
