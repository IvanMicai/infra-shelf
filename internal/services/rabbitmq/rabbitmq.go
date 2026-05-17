// Package rabbitmq provisions per-app vhosts, users and permissions inside
// the shared `infra-rabbitmq` container. Backup exports the rabbitmqctl
// definitions filtered by vhost; restore feeds the JSON back through
// `rabbitmqctl import_definitions`.
package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ivan/infra-shelf/internal/docker"
	"github.com/ivan/infra-shelf/internal/passwordgen"
	"github.com/ivan/infra-shelf/internal/registry"
)

const Container = "infra-rabbitmq"

func Provision(ctx context.Context, appName string) (registry.RabbitMQConfig, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return registry.RabbitMQConfig{}, err
	}
	password := passwordgen.Generate(24)

	steps := [][]string{
		{"rabbitmqctl", "add_vhost", appName},
		{"rabbitmqctl", "add_user", appName, password},
		{"rabbitmqctl", "set_permissions", "-p", appName, appName, ".*", ".*", ".*"},
		{"rabbitmqctl", "set_user_tags", appName, "management"},
	}
	for _, args := range steps {
		if _, err := docker.Exec(ctx, Container, args...); err != nil {
			return registry.RabbitMQConfig{}, err
		}
	}

	return registry.RabbitMQConfig{
		Vhost:    appName,
		Username: appName,
		Password: password,
	}, nil
}

// definitions is the subset of rabbitmqctl export_definitions output we
// preserve in a backup. Filtering by vhost makes the file safe to import even
// into a different broker.
type definitions struct {
	RabbitVersion any              `json:"rabbit_version,omitempty"`
	Vhosts        []map[string]any `json:"vhosts"`
	Users         []map[string]any `json:"users"`
	Permissions   []map[string]any `json:"permissions"`
	Queues        []map[string]any `json:"queues"`
	Exchanges     []map[string]any `json:"exchanges"`
	Bindings      []map[string]any `json:"bindings"`
	Policies      []map[string]any `json:"policies"`
}

func Backup(ctx context.Context, appName, destPath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	raw, err := docker.Exec(ctx, Container, "rabbitmqctl", "export_definitions", "-")
	if err != nil {
		return err
	}
	var full map[string]any
	if err := json.Unmarshal([]byte(raw), &full); err != nil {
		return fmt.Errorf("rabbitmq backup: parse definitions: %w", err)
	}

	filtered := definitions{
		RabbitVersion: full["rabbit_version"],
		Vhosts:        filterByKey(full["vhosts"], "name", appName),
		Users:         filterByKey(full["users"], "name", appName),
		Permissions:   filterByKey(full["permissions"], "vhost", appName),
		Queues:        filterByKey(full["queues"], "vhost", appName),
		Exchanges:     filterByKey(full["exchanges"], "vhost", appName),
		Bindings:      filterByKey(full["bindings"], "vhost", appName),
		Policies:      filterByKey(full["policies"], "vhost", appName),
	}

	payload, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, payload, 0o600)
}

func filterByKey(raw any, key, value string) []map[string]any {
	out := []map[string]any{}
	items, ok := raw.([]any)
	if !ok {
		return out
	}
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if v, _ := m[key].(string); v == value {
			out = append(out, m)
		}
	}
	return out
}

// Restore pipes the JSON definitions file into rabbitmqctl import_definitions.
// Idempotent: existing objects are updated rather than re-created.
func Restore(ctx context.Context, _ string, srcPath string) error {
	payload, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if _, err := docker.ExecWithStdin(ctx, Container, bytes.NewReader(payload),
		"rabbitmqctl", "import_definitions", "/dev/stdin",
	); err != nil {
		return fmt.Errorf("rabbitmqctl import failed: %w", err)
	}
	return nil
}

func Teardown(ctx context.Context, appName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	// Both deletes are best-effort: a missing user is not fatal for the
	// overall removal flow (we still want vhost gone), and vice-versa.
	_, _ = docker.Exec(ctx, Container, "rabbitmqctl", "delete_user", appName)
	_, err := docker.Exec(ctx, Container, "rabbitmqctl", "delete_vhost", appName)
	return err
}
