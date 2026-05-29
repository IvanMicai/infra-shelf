package shelfcore

import (
	"context"
	"fmt"

	"github.com/IvanMicai/infra-shelf/internal/envspec"
	"github.com/IvanMicai/infra-shelf/internal/registry"
)

type AddOptions struct {
	Services []string
	Envs     []string
	Env      string
}

// AddServices attaches services to existing app(s). Services already
// provisioned on a target are skipped with a warning. Mirrors `shelf add`.
func (e *Engine) AddServices(ctx context.Context, appName string, opts AddOptions) ([]TargetResult, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return nil, err
	}
	if len(opts.Services) == 0 {
		return nil, ErrNoServices
	}
	if _, err := registry.ParseServices(opts.Services); err != nil {
		return nil, err
	}

	reg, err := e.Store.Load()
	if err != nil {
		return nil, err
	}

	targets := envspec.BuildAddTargets(appName, envspec.Options{Envs: opts.Envs, Env: opts.Env}, reg)

	for _, target := range targets {
		if _, ok := reg.Apps[target.Name]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrAppNotFound, target.Name)
		}
	}

	if err := containerCheck(ctx, opts.Services); err != nil {
		return nil, err
	}

	results := make([]TargetResult, 0, len(targets))
	for _, target := range targets {
		entry := reg.Apps[target.Name]
		result := TargetResult{Name: target.Name, Failures: map[string]error{}}

		toProvision := []string{}
		for _, svc := range opts.Services {
			if hasService(entry, svc) {
				e.Reporter.Warn(fmt.Sprintf("%s: %s already provisioned — skipping", target.Name, svc))
				continue
			}
			toProvision = append(toProvision, svc)
		}

		if len(toProvision) == 0 {
			e.Reporter.Info(fmt.Sprintf("%s: nothing to do", target.Name))
			result.Entry = entry
			results = append(results, result)
			continue
		}

		provOpts := provisionOptions{
			SignozServiceName: target.SignozServiceName,
			SignozEnvironment: target.SignozEnv,
		}

		for _, svc := range toProvision {
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
