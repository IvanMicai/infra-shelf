package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/IvanMicai/infra-shelf/internal/config"
	"github.com/IvanMicai/infra-shelf/internal/output"
	"github.com/IvanMicai/infra-shelf/internal/registry"
	"github.com/IvanMicai/infra-shelf/internal/shelfcore"
)

// cliReporter adapts internal/output to shelfcore.Reporter so the engine can
// stream progress straight to the user's terminal.
type cliReporter struct{}

func (cliReporter) Success(m string) { output.Success(m) }
func (cliReporter) Error(m string)   { output.Error(m) }
func (cliReporter) Info(m string)    { output.Info(m) }
func (cliReporter) Warn(m string)    { output.Warn(m) }
func (cliReporter) Title(m string)   { output.Title(m) }

// buildEngine loads config and constructs a shelfcore.Engine wired to the CLI
// reporter. Returns an error suitable to bubble out of a cobra RunE.
func buildEngine() (*shelfcore.Engine, config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, config.Config{}, fmt.Errorf("config: %w", err)
	}
	store := registry.NewStore(cfg.RegistryPath)
	return shelfcore.New(store, cfg.BackupsDir, cliReporter{}), cfg, nil
}

// parseServicesFlag splits a comma-separated --services value, validates
// against the known service catalog, and returns a deduplicated slice.
func parseServicesFlag(cmd *cobra.Command) ([]string, error) {
	raw, _ := cmd.Flags().GetString("services")
	parts := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return registry.ParseServices(parts)
}

// requireAtLeastOneService surfaces a CLI-friendly error when -s is missing,
// without leaking shelfcore's internal sentinel into the error message.
func requireAtLeastOneService(svcs []string, hint string) error {
	if len(svcs) > 0 {
		return nil
	}
	return fmt.Errorf("at least one service is required (use -s %s)", hint)
}

// isSentinel reports whether err is one of shelfcore's tagged errors; used by
// command handlers to format messages without changing exit codes.
func isSentinel(err error, sentinel error) bool {
	return errors.Is(err, sentinel)
}
