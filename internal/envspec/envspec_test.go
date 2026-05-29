package envspec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/IvanMicai/infra-shelf/internal/registry"
)

func TestParseEnvs(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		got, err := ParseEnvs("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("single value", func(t *testing.T) {
		got, err := ParseEnvs("staging")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"staging"}) {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("csv expands", func(t *testing.T) {
		got, err := ParseEnvs("staging,production")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"staging", "production"}) {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		got, err := ParseEnvs("  staging , production ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"staging", "production"}) {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("rejects all-empty", func(t *testing.T) {
		_, err := ParseEnvs(" , , ")
		if err == nil || !strings.Contains(err.Error(), "at least one") {
			t.Fatalf("expected 'at least one' error, got %v", err)
		}
	})

	t.Run("rejects invalid name", func(t *testing.T) {
		if _, err := ParseEnvs("Staging"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := ParseEnvs("staging,Prod"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects duplicates", func(t *testing.T) {
		_, err := ParseEnvs("staging,staging")
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	})
}

func TestParseSingleEnv(t *testing.T) {
	t.Run("empty returns empty", func(t *testing.T) {
		got, err := ParseSingleEnv("")
		if err != nil || got != "" {
			t.Fatalf("got %q, %v", got, err)
		}
		got, err = ParseSingleEnv("   ")
		if err != nil || got != "" {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("valid passes through", func(t *testing.T) {
		got, err := ParseSingleEnv("staging")
		if err != nil || got != "staging" {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("trims", func(t *testing.T) {
		got, _ := ParseSingleEnv("  staging  ")
		if got != "staging" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("rejects invalid", func(t *testing.T) {
		if _, err := ParseSingleEnv("Staging"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := ParseSingleEnv("staging,production"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestBuildSetupTargets(t *testing.T) {
	t.Run("no flags", func(t *testing.T) {
		got := BuildSetupTargets("iara", Options{})
		want := []Target{{Name: "iara", SignozServiceName: "iara"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("--env tags single", func(t *testing.T) {
		got := BuildSetupTargets("teste5-staging", Options{Env: "staging"})
		want := []Target{{Name: "teste5-staging", SignozServiceName: "teste5-staging", SignozEnv: "staging"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("--envs expands", func(t *testing.T) {
		got := BuildSetupTargets("iara", Options{Envs: []string{"staging", "production"}})
		want := []Target{
			{Name: "iara-staging", SignozServiceName: "iara", SignozEnv: "staging"},
			{Name: "iara-production", SignozServiceName: "iara", SignozEnv: "production"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("--envs single still expands", func(t *testing.T) {
		got := BuildSetupTargets("iara", Options{Envs: []string{"staging"}})
		want := []Target{{Name: "iara-staging", SignozServiceName: "iara", SignozEnv: "staging"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("empty envs slice falls back to default", func(t *testing.T) {
		got := BuildSetupTargets("iara", Options{Envs: []string{}})
		want := []Target{{Name: "iara", SignozServiceName: "iara"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})
}

func TestBuildAddTargets(t *testing.T) {
	mkReg := func(apps map[string]registry.AppEntry) registry.Registry {
		if apps == nil {
			apps = map[string]registry.AppEntry{}
		}
		return registry.Registry{Version: 1, Apps: apps}
	}

	t.Run("no flags, registry empty", func(t *testing.T) {
		got := BuildAddTargets("iara", Options{}, mkReg(map[string]registry.AppEntry{
			"iara": {CreatedAt: "x"},
		}))
		want := []Target{{Name: "iara", SignozServiceName: "iara"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("falls back to entry.environment", func(t *testing.T) {
		got := BuildAddTargets("iara-staging", Options{}, mkReg(map[string]registry.AppEntry{
			"iara-staging": {CreatedAt: "x", Environment: "staging", SignozServiceName: "iara"},
		}))
		want := []Target{{Name: "iara-staging", SignozServiceName: "iara", SignozEnv: "staging"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("--env overrides persisted env", func(t *testing.T) {
		got := BuildAddTargets("iara-staging", Options{Env: "production"}, mkReg(map[string]registry.AppEntry{
			"iara-staging": {CreatedAt: "x", Environment: "staging", SignozServiceName: "iara"},
		}))
		want := []Target{{Name: "iara-staging", SignozServiceName: "iara", SignozEnv: "production"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("--envs ignores persisted state", func(t *testing.T) {
		got := BuildAddTargets("iara", Options{Envs: []string{"staging", "production"}}, mkReg(map[string]registry.AppEntry{
			"iara-staging":    {CreatedAt: "x"},
			"iara-production": {CreatedAt: "x"},
		}))
		want := []Target{
			{Name: "iara-staging", SignozServiceName: "iara", SignozEnv: "staging"},
			{Name: "iara-production", SignozServiceName: "iara", SignozEnv: "production"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("missing entry → service name = app name", func(t *testing.T) {
		got := BuildAddTargets("ghost", Options{}, mkReg(nil))
		want := []Target{{Name: "ghost", SignozServiceName: "ghost"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})
}
