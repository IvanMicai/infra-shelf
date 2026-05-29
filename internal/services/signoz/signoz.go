// Package signoz produces OTEL configuration for an app. SignOz Community
// has no per-app tenancy — provisioning is config-only.
package signoz

import (
	"os"
	"strings"

	"github.com/IvanMicai/infra-shelf/internal/registry"
)

type ProvisionOptions struct {
	ServiceName string // override (defaults to appName)
	Environment string // override (defaults to SIGNOZ_DEFAULT_ENV or "dev")
}

func Provision(appName string, opts ProvisionOptions) registry.SignozConfig {
	serviceName := opts.ServiceName
	if serviceName == "" {
		serviceName = appName
	}
	environment := opts.Environment
	if environment == "" {
		if env := strings.TrimSpace(os.Getenv("SIGNOZ_DEFAULT_ENV")); env != "" {
			environment = env
		} else {
			environment = "dev"
		}
	}
	return registry.SignozConfig{ServiceName: serviceName, Environment: environment}
}

// Teardown is intentionally a no-op: SignOz Community shares ClickHouse across
// all apps and offers no per-app resources to delete. Registry entry removal
// is enough; telemetry ages out via the SignOz retention policy.
func Teardown(_ string) error { return nil }
