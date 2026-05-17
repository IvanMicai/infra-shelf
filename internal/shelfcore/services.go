package shelfcore

import (
	"context"
	"fmt"

	"github.com/ivan/infra-shelf/internal/docker"
	"github.com/ivan/infra-shelf/internal/registry"
	"github.com/ivan/infra-shelf/internal/services/aistor"
	"github.com/ivan/infra-shelf/internal/services/postgres"
	"github.com/ivan/infra-shelf/internal/services/rabbitmq"
	"github.com/ivan/infra-shelf/internal/services/redis"
)

// serviceContainer maps each provisionable service to the container name where
// its admin tooling (psql, redis-cli, rabbitmqctl, mc) lives.
var serviceContainer = map[string]string{
	"postgres": postgres.Container,
	"redis":    redis.Container,
	"rabbitmq": rabbitmq.Container,
	"aistor":   aistor.Container,
	"signoz":   "infra-signoz-otel-collector",
}

// startHint tells the user how to bring up a missing container. SignOz lives
// in the optional compose overlay, so it has a different command.
var startHint = map[string]string{
	"signoz": "make signoz-up",
}

// nonBackupable lists services that are intentionally skipped during backup
// flows (no per-app data — telemetry only).
var nonBackupable = map[string]bool{
	"signoz": true,
}

// detachable lists services that `shelf detach` is allowed to remove without
// invoking a teardown. Only addons (no per-app credentials) qualify.
var detachable = map[string]bool{
	"signoz": true,
}

// containerCheck verifies the given services have their backing container in
// the running state. Returns the first failure with ErrContainerNotRunning
// wrapped, so callers can decide whether to surface it as fatal or skip.
func containerCheck(ctx context.Context, services []string) error {
	for _, svc := range services {
		container := serviceContainer[svc]
		if !docker.IsContainerRunning(ctx, container) {
			hint := startHint[svc]
			if hint == "" {
				hint = "make up"
			}
			return fmt.Errorf("%w: container %q for %s (start with: %s)",
				ErrContainerNotRunning, container, svc, hint)
		}
	}
	return nil
}

func teardownService(ctx context.Context, svc string, appName string) error {
	switch svc {
	case "postgres":
		return postgres.Teardown(ctx, appName)
	case "redis":
		return redis.Teardown(ctx, appName)
	case "rabbitmq":
		return rabbitmq.Teardown(ctx, appName)
	case "aistor":
		return aistor.Teardown(ctx, appName)
	case "signoz":
		return nil
	}
	return fmt.Errorf("unknown service: %s", svc)
}

func setServiceConfig(entry *registry.AppEntry, svc string, cfg any) {
	switch svc {
	case "postgres":
		c := cfg.(registry.PostgresConfig)
		entry.Services.Postgres = &c
	case "redis":
		c := cfg.(registry.RedisConfig)
		entry.Services.Redis = &c
	case "rabbitmq":
		c := cfg.(registry.RabbitMQConfig)
		entry.Services.RabbitMQ = &c
	case "aistor":
		c := cfg.(registry.AIStorConfig)
		entry.Services.AIStor = &c
	case "signoz":
		c := cfg.(registry.SignozConfig)
		entry.Services.Signoz = &c
	}
}

func clearServiceConfig(entry *registry.AppEntry, svc string) {
	switch svc {
	case "postgres":
		entry.Services.Postgres = nil
	case "redis":
		entry.Services.Redis = nil
	case "rabbitmq":
		entry.Services.RabbitMQ = nil
	case "aistor":
		entry.Services.AIStor = nil
	case "signoz":
		entry.Services.Signoz = nil
	}
}

func hasService(entry registry.AppEntry, svc string) bool {
	switch svc {
	case "postgres":
		return entry.Services.Postgres != nil
	case "redis":
		return entry.Services.Redis != nil
	case "rabbitmq":
		return entry.Services.RabbitMQ != nil
	case "aistor":
		return entry.Services.AIStor != nil
	case "signoz":
		return entry.Services.Signoz != nil
	}
	return false
}

// provisionService dispatches to the right service.Provision and returns the
// envelope-typed config so callers can stash it into the registry entry.
func provisionService(ctx context.Context, svc string, appName string, opts provisionOptions) (any, error) {
	switch svc {
	case "postgres":
		return postgres.Provision(ctx, appName)
	case "redis":
		return redis.Provision(ctx, appName, redis.ProvisionOptions{FullAccess: opts.FullAccess})
	case "rabbitmq":
		return rabbitmq.Provision(ctx, appName)
	case "aistor":
		return aistor.Provision(ctx, appName)
	case "signoz":
		return signozProvision(appName, opts), nil
	}
	return nil, fmt.Errorf("unknown service: %s", svc)
}

type provisionOptions struct {
	FullAccess        bool
	SignozServiceName string
	SignozEnvironment string
}

// signozProvision is split out because signoz has no async work; we still
// route through provisionService for symmetry.
func signozProvision(appName string, opts provisionOptions) registry.SignozConfig {
	// Indirect call into services/signoz to keep dependency direction clean.
	return signozAdapter(appName, opts.SignozServiceName, opts.SignozEnvironment)
}
