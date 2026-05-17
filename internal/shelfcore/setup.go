package shelfcore

import (
	"context"
	"fmt"
	"time"

	"github.com/ivan/infra-shelf/internal/envspec"
	"github.com/ivan/infra-shelf/internal/registry"
)

type SetupOptions struct {
	Services   []string
	Envs       []string // expand into siblings <app>-<env>
	Env        string   // tag a single app
	FullAccess bool     // redis: grant ~* instead of <app>:*
}

// TargetResult captures what happened to one materialized app (one entry in
// the result of SetupApp/AddServices). Errors per-service are non-fatal — they
// don't abort the rest of the targets, mirroring the legacy TS behavior.
type TargetResult struct {
	Name              string
	Entry             registry.AppEntry
	Provisioned       []string                  // services successfully provisioned
	Failures          map[string]error          // service → error
}

// SetupApp creates one or more apps (depending on --envs) and provisions
// services on each. Returns one TargetResult per materialized app.
//
// Fail-fast: if any target already exists or any required container is down,
// nothing is mutated.
func (e *Engine) SetupApp(ctx context.Context, appName string, opts SetupOptions) ([]TargetResult, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return nil, err
	}
	if len(opts.Services) == 0 {
		return nil, ErrNoServices
	}
	if _, err := registry.ParseServices(opts.Services); err != nil {
		return nil, err
	}

	targets := envspec.BuildSetupTargets(appName, envspec.Options{Envs: opts.Envs, Env: opts.Env})

	reg, err := e.Store.Load()
	if err != nil {
		return nil, err
	}

	for _, target := range targets {
		if err := registry.ValidateAppName(target.Name); err != nil {
			return nil, fmt.Errorf("invalid expanded app name %q (check --envs values): %w", target.Name, err)
		}
		if _, exists := reg.Apps[target.Name]; exists {
			return nil, fmt.Errorf("%w: %s", ErrAppAlreadyExists, target.Name)
		}
	}

	if err := containerCheck(ctx, opts.Services); err != nil {
		return nil, err
	}

	results := make([]TargetResult, 0, len(targets))
	for _, target := range targets {
		entry := registry.AppEntry{
			CreatedAt:   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
			Environment: target.SignozEnv,
		}
		if target.SignozServiceName != target.Name {
			entry.SignozServiceName = target.SignozServiceName
		}

		result := TargetResult{
			Name:     target.Name,
			Failures: map[string]error{},
		}

		provOpts := provisionOptions{
			FullAccess:        opts.FullAccess,
			SignozServiceName: target.SignozServiceName,
			SignozEnvironment: target.SignozEnv,
		}

		for _, svc := range opts.Services {
			cfg, err := provisionService(ctx, svc, target.Name, provOpts)
			if err != nil {
				e.Reporter.Error(fmt.Sprintf("failed to provision %s for %s: %v", svc, target.Name, err))
				result.Failures[svc] = err
				continue
			}
			setServiceConfig(&entry, svc, cfg)
			result.Provisioned = append(result.Provisioned, svc)
			e.Reporter.Success(fmt.Sprintf("%s: %s provisioned", target.Name, svc))
		}

		reg.Apps[target.Name] = entry
		if err := e.Store.Save(reg); err != nil {
			return results, fmt.Errorf("save registry: %w", err)
		}

		result.Entry = entry
		results = append(results, result)
	}

	return results, nil
}
