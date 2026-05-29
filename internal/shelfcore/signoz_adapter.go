package shelfcore

import (
	"github.com/IvanMicai/infra-shelf/internal/registry"
	"github.com/IvanMicai/infra-shelf/internal/services/signoz"
)

// signozAdapter wraps signoz.Provision so services.go can call it without
// importing the package directly (avoids a circular dependency once we add
// more signoz config sources here).
func signozAdapter(appName, serviceName, environment string) registry.SignozConfig {
	return signoz.Provision(appName, signoz.ProvisionOptions{
		ServiceName: serviceName,
		Environment: environment,
	})
}
