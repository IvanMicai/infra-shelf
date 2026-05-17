package shelfcore

import (
	"context"
	"fmt"

	"github.com/ivan/infra-shelf/internal/registry"
)

// DetachServices removes addon services (currently only signoz) from an app's
// registry entry without tearing down container resources. Non-detachable
// services (postgres/redis/rabbitmq/aistor) are rejected with ErrNotDetachable
// — callers should use RemoveApp instead.
func (e *Engine) DetachServices(ctx context.Context, appName string, services []string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	if len(services) == 0 {
		return ErrNoServices
	}
	if _, err := registry.ParseServices(services); err != nil {
		return err
	}

	for _, svc := range services {
		if !detachable[svc] {
			return fmt.Errorf("%w: %s (use `shelf remove` for full teardown)", ErrNotDetachable, svc)
		}
	}

	reg, err := e.Store.Load()
	if err != nil {
		return err
	}
	entry, ok := reg.Apps[appName]
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appName)
	}

	for _, svc := range services {
		if !hasService(entry, svc) {
			e.Reporter.Warn(fmt.Sprintf("%s is not attached to %q — skipping", svc, appName))
			continue
		}
		if err := teardownService(ctx, svc, appName); err != nil {
			return fmt.Errorf("teardown %s: %w", svc, err)
		}
		clearServiceConfig(&entry, svc)
		e.Reporter.Success(fmt.Sprintf("%s detached from %s", svc, appName))
	}

	reg.Apps[appName] = entry
	return e.Store.Save(reg)
}
