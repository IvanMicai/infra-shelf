package shelfcore

import (
	"context"
	"fmt"
	"sort"

	"github.com/IvanMicai/infra-shelf/internal/services/mongodb"
	"github.com/IvanMicai/infra-shelf/internal/services/rabbitmq"
)

// ReconcileResult summarizes what Reconcile did for one app.
type ReconcileResult struct {
	App      string
	Ensured  []string         // services that were re-applied successfully
	Failures map[string]error // service → error (non-fatal, other apps continue)
}

// Reconcile walks the registry and re-applies every provisioned per-app
// resource against the live containers. It is the recovery hatch for the
// case where a data container's volume was wiped (server reset, accidental
// volume delete, etc.) — the registry remembers the credentials, Reconcile
// pushes them back into the broker / DB.
//
// Currently covers RabbitMQ and MongoDB (both keep per-app credentials in the
// data volume). Postgres / Redis / AIstor can be added under the same pattern
// (service-level Ensure + a branch below).
//
// Fail-soft: a failure on one app/service is recorded in Failures but does
// not abort the rest. Useful for `docker compose up` one-shots where partial
// success still leaves the system better off than before.
func (e *Engine) Reconcile(ctx context.Context) ([]ReconcileResult, error) {
	reg, err := e.Store.Load()
	if err != nil {
		return nil, fmt.Errorf("reconcile: load registry: %w", err)
	}

	names := make([]string, 0, len(reg.Apps))
	for name := range reg.Apps {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]ReconcileResult, 0, len(names))
	for _, name := range names {
		entry := reg.Apps[name]
		res := ReconcileResult{App: name, Failures: map[string]error{}}

		if entry.Services.RabbitMQ != nil {
			cfg := *entry.Services.RabbitMQ
			if err := rabbitmq.Ensure(ctx, cfg); err != nil {
				e.Reporter.Error(fmt.Sprintf("[%s] rabbitmq: %v", name, err))
				res.Failures["rabbitmq"] = err
			} else {
				e.Reporter.Success(fmt.Sprintf("[%s] rabbitmq vhost=%s user=%s", name, cfg.Vhost, cfg.Username))
				res.Ensured = append(res.Ensured, "rabbitmq")
			}
		}

		if entry.Services.MongoDB != nil {
			cfg := *entry.Services.MongoDB
			if err := mongodb.Ensure(ctx, cfg); err != nil {
				e.Reporter.Error(fmt.Sprintf("[%s] mongodb: %v", name, err))
				res.Failures["mongodb"] = err
			} else {
				e.Reporter.Success(fmt.Sprintf("[%s] mongodb db=%s user=%s", name, cfg.Database, cfg.Username))
				res.Ensured = append(res.Ensured, "mongodb")
			}
		}

		results = append(results, res)
	}
	return results, nil
}
