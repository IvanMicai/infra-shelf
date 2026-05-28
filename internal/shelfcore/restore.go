package shelfcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ivan/infra-shelf/internal/docker"
	"github.com/ivan/infra-shelf/internal/registry"
	"github.com/ivan/infra-shelf/internal/services/aistor"
	"github.com/ivan/infra-shelf/internal/services/mongodb"
	"github.com/ivan/infra-shelf/internal/services/postgres"
	"github.com/ivan/infra-shelf/internal/services/rabbitmq"
	"github.com/ivan/infra-shelf/internal/services/redis"
)

type RestoreOptions struct {
	Services []string // empty = all backupable services on the app
}

// RestorePlanEntry describes one file the engine will restore from. Useful for
// dry-run confirmations in the CLI and for response payloads in the web UI.
type RestorePlanEntry struct {
	Service string
	File    string
}

// PlanRestore computes which backup files would be applied if RestoreApp were
// called with the same arguments. It does not touch any container.
func (e *Engine) PlanRestore(_ context.Context, appName string, opts RestoreOptions) ([]RestorePlanEntry, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return nil, err
	}

	reg, err := e.Store.Load()
	if err != nil {
		return nil, err
	}
	entry, ok := reg.Apps[appName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAppNotFound, appName)
	}

	targets := chooseBackupServices(entry, opts.Services)
	appDir := filepath.Join(e.BackupsDir, appName)

	plan := []RestorePlanEntry{}
	for _, svc := range targets {
		latest := findLatestBackup(appDir, svc)
		if latest == "" {
			e.Reporter.Warn(fmt.Sprintf("no backup found for %s", svc))
			continue
		}
		plan = append(plan, RestorePlanEntry{Service: svc, File: latest})
	}
	if len(plan) == 0 {
		return nil, ErrBackupNotFound
	}
	return plan, nil
}

// RestoreApp restores the latest backup of each service in opts.Services
// (defaulting to every backupable service the app has provisioned). Use
// RestoreFromFile to restore a specific file.
func (e *Engine) RestoreApp(ctx context.Context, appName string, opts RestoreOptions) error {
	plan, err := e.PlanRestore(ctx, appName, opts)
	if err != nil {
		return err
	}

	for _, p := range plan {
		container := serviceContainer[p.Service]
		if !docker.IsContainerRunning(ctx, container) {
			e.Reporter.Error(fmt.Sprintf("container %q is not running. Skipping %s", container, p.Service))
			continue
		}
		if err := runRestore(ctx, p.Service, appName, p.File); err != nil {
			e.Reporter.Error(fmt.Sprintf("failed to restore %s: %v", p.Service, err))
			continue
		}
		e.Reporter.Success(fmt.Sprintf("%s restored", p.Service))
	}
	return nil
}

// RestoreFromFile restores a single backup file. The service is detected from
// the file name prefix (postgres_, redis_, rabbitmq_, aistor_).
func (e *Engine) RestoreFromFile(ctx context.Context, appName, filePath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}

	reg, err := e.Store.Load()
	if err != nil {
		return err
	}
	if _, ok := reg.Apps[appName]; !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appName)
	}

	resolved, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(resolved); err != nil {
		return fmt.Errorf("file not found: %s", resolved)
	}

	fileName := filepath.Base(resolved)
	svc := detectService(fileName)
	if svc == "" {
		return fmt.Errorf("cannot detect service from filename %q (expected postgres_*, redis_*, rabbitmq_*, aistor_*, mongodb_*)", fileName)
	}

	if !docker.IsContainerRunning(ctx, serviceContainer[svc]) {
		return fmt.Errorf("%w: container %q for %s", ErrContainerNotRunning, serviceContainer[svc], svc)
	}

	if err := runRestore(ctx, svc, appName, resolved); err != nil {
		return err
	}
	e.Reporter.Success(fmt.Sprintf("%s restored from %s", svc, fileName))
	return nil
}

func runRestore(ctx context.Context, svc, appName, file string) error {
	switch svc {
	case "postgres":
		return postgres.Restore(ctx, appName, file)
	case "redis":
		return redis.Restore(ctx, appName, file)
	case "rabbitmq":
		return rabbitmq.Restore(ctx, appName, file)
	case "aistor":
		return aistor.Restore(ctx, appName, file)
	case "mongodb":
		return mongodb.Restore(ctx, appName, file)
	}
	return fmt.Errorf("unknown service: %s", svc)
}

func detectService(fileName string) string {
	for _, svc := range []string{"postgres", "redis", "rabbitmq", "aistor", "mongodb"} {
		if strings.HasPrefix(fileName, svc+"_") {
			return svc
		}
	}
	return ""
}

func findLatestBackup(appDir, svc string) string {
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return ""
	}
	matching := []string{}
	prefix := svc + "_"
	for _, ent := range entries {
		if !ent.IsDir() && strings.HasPrefix(ent.Name(), prefix) {
			matching = append(matching, ent.Name())
		}
	}
	if len(matching) == 0 {
		return ""
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matching)))
	return filepath.Join(appDir, matching[0])
}
