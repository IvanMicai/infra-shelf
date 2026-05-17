package shelfcore

import (
	"fmt"

	"github.com/ivan/infra-shelf/internal/registry"
)

// Credentials returns the rendered .env block for an app. The web UI reveals
// it on demand; the CLI prints it directly to stdout. Empty string when the
// app has no services attached.
func (e *Engine) Credentials(appName string) (string, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return "", err
	}
	app, found, err := e.Store.GetApp(appName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrAppNotFound, appName)
	}
	return app.EnvFile(), nil
}

// ListApps proxies the registry's ListApps in case the web/CLI both want one
// API surface for everything.
func (e *Engine) ListApps() ([]registry.App, error) {
	return e.Store.ListApps()
}
