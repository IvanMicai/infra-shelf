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
	"strings"

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

// Ensure idempotently re-applies vhost/user/perms for an existing app entry.
// Used by reconcile loop after the rabbitmq volume is lost: vhost is created
// if missing, user has its password reset to the registered one, perms are
// re-applied (rabbitmqctl set_permissions overwrites). Safe to call repeatedly.
func Ensure(ctx context.Context, cfg registry.RabbitMQConfig) error {
	if cfg.Vhost == "" || cfg.Username == "" || cfg.Password == "" {
		return fmt.Errorf("rabbitmq ensure: incomplete config (vhost=%q user=%q)", cfg.Vhost, cfg.Username)
	}

	// vhost: idempotent — `add_vhost` on existing vhost is a no-op (rabbit
	// returns success). Use list_vhosts to be safe across rabbit versions.
	vhosts, err := docker.Exec(ctx, Container, "rabbitmqctl", "list_vhosts", "--no-table-headers")
	if err != nil {
		return fmt.Errorf("rabbitmq ensure: list_vhosts: %w", err)
	}
	if !containsLine(vhosts, cfg.Vhost) {
		if _, err := docker.Exec(ctx, Container, "rabbitmqctl", "add_vhost", cfg.Vhost); err != nil {
			return fmt.Errorf("rabbitmq ensure: add_vhost %q: %w", cfg.Vhost, err)
		}
	}

	// user: create if missing, otherwise reset password to the registered value.
	users, err := docker.Exec(ctx, Container, "rabbitmqctl", "list_users", "--no-table-headers")
	if err != nil {
		return fmt.Errorf("rabbitmq ensure: list_users: %w", err)
	}
	if !containsUser(users, cfg.Username) {
		if _, err := docker.Exec(ctx, Container, "rabbitmqctl", "add_user", cfg.Username, cfg.Password); err != nil {
			return fmt.Errorf("rabbitmq ensure: add_user %q: %w", cfg.Username, err)
		}
	} else {
		if _, err := docker.Exec(ctx, Container, "rabbitmqctl", "change_password", cfg.Username, cfg.Password); err != nil {
			return fmt.Errorf("rabbitmq ensure: change_password %q: %w", cfg.Username, err)
		}
	}

	// perms + tags: set_* overwrites, always safe.
	if _, err := docker.Exec(ctx, Container, "rabbitmqctl", "set_permissions", "-p", cfg.Vhost, cfg.Username, ".*", ".*", ".*"); err != nil {
		return fmt.Errorf("rabbitmq ensure: set_permissions: %w", err)
	}
	if _, err := docker.Exec(ctx, Container, "rabbitmqctl", "set_user_tags", cfg.Username, "management"); err != nil {
		return fmt.Errorf("rabbitmq ensure: set_user_tags: %w", err)
	}
	return nil
}

func containsLine(out, target string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

// rabbitmqctl list_users emits "<name>\t[tag,...]" — match on the first field.
func containsUser(out, target string) bool {
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.SplitN(line, "\t", 2)[0])
		if name == target {
			return true
		}
	}
	return false
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
