// Package envspec parses the --env / --envs flags and computes the apps each
// shelf command should touch. Pure functions, no I/O.
package envspec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/IvanMicai/infra-shelf/internal/registry"
)

var envNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ParseEnvs parses a CSV like "staging,production" into a slice. Returns nil
// when raw is empty (flag not provided). Errors on empty parts, invalid names,
// or duplicates.
func ParseEnvs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	parts := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("--envs requires at least one environment name")
	}
	seen := map[string]bool{}
	for _, env := range parts {
		if !envNamePattern.MatchString(env) {
			return nil, fmt.Errorf("invalid env name %q. Use lowercase letters, numbers, and hyphens", env)
		}
		if seen[env] {
			return nil, fmt.Errorf("duplicate envs in --envs: %s", strings.Join(parts, ","))
		}
		seen[env] = true
	}
	return parts, nil
}

// ParseSingleEnv parses --env (singular). Returns "" when raw is empty.
func ParseSingleEnv(raw string) (string, error) {
	env := strings.TrimSpace(raw)
	if env == "" {
		return "", nil
	}
	if !envNamePattern.MatchString(env) {
		return "", fmt.Errorf("invalid env name %q. Use lowercase letters, numbers, and hyphens", env)
	}
	return env, nil
}

// Target describes one app a command will materialize.
type Target struct {
	Name              string
	SignozServiceName string
	SignozEnv         string
}

// Options carries the parsed --env/--envs values.
type Options struct {
	Envs []string
	Env  string
}

// BuildSetupTargets computes the targets for `shelf setup`. With --envs it
// expands into sibling apps `<app>-<env>` all sharing serviceName=<app>; with
// --env it tags a single app; otherwise a single bare app.
func BuildSetupTargets(appName string, opts Options) []Target {
	if len(opts.Envs) > 0 {
		out := make([]Target, 0, len(opts.Envs))
		for _, env := range opts.Envs {
			out = append(out, Target{
				Name:              fmt.Sprintf("%s-%s", appName, env),
				SignozServiceName: appName,
				SignozEnv:         env,
			})
		}
		return out
	}
	return []Target{{
		Name:              appName,
		SignozServiceName: appName,
		SignozEnv:         opts.Env,
	}}
}

// BuildAddTargets computes targets for `shelf add`. When neither --envs nor
// --env is given, it pulls signozServiceName and environment from the existing
// registry entry so reattaching an addon preserves the original setup env.
func BuildAddTargets(appName string, opts Options, reg registry.Registry) []Target {
	if len(opts.Envs) > 0 {
		out := make([]Target, 0, len(opts.Envs))
		for _, env := range opts.Envs {
			out = append(out, Target{
				Name:              fmt.Sprintf("%s-%s", appName, env),
				SignozServiceName: appName,
				SignozEnv:         env,
			})
		}
		return out
	}

	signozServiceName := appName
	signozEnv := opts.Env
	if entry, ok := reg.Apps[appName]; ok {
		if entry.SignozServiceName != "" {
			signozServiceName = entry.SignozServiceName
		}
		if signozEnv == "" {
			signozEnv = entry.Environment
		}
	}
	return []Target{{
		Name:              appName,
		SignozServiceName: signozServiceName,
		SignozEnv:         signozEnv,
	}}
}
