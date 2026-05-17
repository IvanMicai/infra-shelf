package shelfcore

import (
	"context"
	"fmt"

	"github.com/ivan/infra-shelf/internal/registry"
)

// orderedServices is the teardown order: addons first (they're cheapest),
// then provisioning services. The order matches the original TS remove.ts
// switch on each services.X check.
var orderedServices = []string{"postgres", "redis", "rabbitmq", "aistor", "signoz"}

// RemoveApp tears down every service attached to appName and deletes the
// registry entry. Individual service teardown errors are reported but don't
// block subsequent steps — leftover resources can be cleaned up manually.
func (e *Engine) RemoveApp(ctx context.Context, appName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}

	reg, err := e.Store.Load()
	if err != nil {
		return err
	}
	entry, ok := reg.Apps[appName]
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appName)
	}

	for _, svc := range orderedServices {
		if !hasService(entry, svc) {
			continue
		}
		if err := teardownService(ctx, svc, appName); err != nil {
			e.Reporter.Error(fmt.Sprintf("failed to remove %s: %v", svc, err))
			continue
		}
		e.Reporter.Success(fmt.Sprintf("%s resources removed", svc))
	}

	delete(reg.Apps, appName)
	if err := e.Store.Save(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}
	e.Reporter.Success(fmt.Sprintf("app %q removed", appName))
	return nil
}
