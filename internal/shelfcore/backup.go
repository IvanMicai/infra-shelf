package shelfcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IvanMicai/infra-shelf/internal/docker"
	"github.com/IvanMicai/infra-shelf/internal/registry"
	"github.com/IvanMicai/infra-shelf/internal/services/aistor"
	"github.com/IvanMicai/infra-shelf/internal/services/mongodb"
	"github.com/IvanMicai/infra-shelf/internal/services/postgres"
	"github.com/IvanMicai/infra-shelf/internal/services/rabbitmq"
	"github.com/IvanMicai/infra-shelf/internal/services/redis"
)

var serviceExt = map[string]string{
	"postgres": "sql",
	"redis":    "json",
	"rabbitmq": "json",
	"aistor":   "tar",
	"mongodb":  "archive",
}

type BackupFile struct {
	App     string
	Service string
	Path    string
}

type BackupOptions struct {
	Services []string // empty = all backupable services on the app
}

// BackupApp dumps the requested services of one app. SignOz is silently
// skipped (non-backupable). The returned BackupFile slice gives callers the
// exact files produced, so they can mirror to S3 / apply retention without
// re-listing the directory.
func (e *Engine) BackupApp(ctx context.Context, appName string, opts BackupOptions) ([]BackupFile, error) {
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
	if len(targets) == 0 {
		return nil, fmt.Errorf("no matching services to backup for %q", appName)
	}

	appDir := filepath.Join(e.BackupsDir, appName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, err
	}

	ts := timestamp()
	out := make([]BackupFile, 0, len(targets))

	for _, svc := range targets {
		container := serviceContainer[svc]
		if !docker.IsContainerRunning(ctx, container) {
			e.Reporter.Error(fmt.Sprintf("container %q is not running. Skipping %s", container, svc))
			continue
		}

		fileName := fmt.Sprintf("%s_%s.%s", svc, ts, serviceExt[svc])
		filePath := filepath.Join(appDir, fileName)

		if err := runBackup(ctx, svc, appName, filePath); err != nil {
			e.Reporter.Error(fmt.Sprintf("failed to backup %s: %v", svc, err))
			// Clean up the empty/partial file so retention/upload doesn't see it.
			_ = os.Remove(filePath)
			continue
		}
		out = append(out, BackupFile{App: appName, Service: svc, Path: filePath})
		e.Reporter.Success(fmt.Sprintf("%s -> %s", svc, fileName))
	}

	return out, nil
}

// BackupAll backs up every provisioned app. Services filter, if provided,
// applies uniformly across all apps. Returns the union of produced files.
func (e *Engine) BackupAll(ctx context.Context, opts BackupOptions) ([]BackupFile, error) {
	reg, err := e.Store.Load()
	if err != nil {
		return nil, err
	}
	if len(reg.Apps) == 0 {
		e.Reporter.Info("no apps provisioned yet")
		return nil, nil
	}

	names := make([]string, 0, len(reg.Apps))
	for name := range reg.Apps {
		names = append(names, name)
	}
	// Deterministic order matches the legacy TS Object.keys iteration on small
	// maps closely enough; sort to make it predictable across runs.
	sortStrings(names)

	all := []BackupFile{}
	for _, name := range names {
		e.Reporter.Title(fmt.Sprintf("backing up %q", name))
		files, err := e.BackupApp(ctx, name, opts)
		if err != nil {
			e.Reporter.Error(err.Error())
			continue
		}
		all = append(all, files...)
	}
	return all, nil
}

// DeleteBackup removes a single backup file from disk. Refuses path traversal
// (fileName must be a base name, not a relative path).
func (e *Engine) DeleteBackup(appName, fileName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	if filepath.Base(fileName) != fileName {
		return fmt.Errorf("refusing path traversal in fileName: %q", fileName)
	}
	path := filepath.Join(e.BackupsDir, appName, fileName)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%w: %s", ErrBackupNotFound, path)
	}
	return os.Remove(path)
}

func chooseBackupServices(entry registry.AppEntry, requested []string) []string {
	provisioned := []string{}
	for _, svc := range orderedServices {
		if !hasService(entry, svc) || nonBackupable[svc] {
			continue
		}
		provisioned = append(provisioned, svc)
	}
	if len(requested) == 0 {
		return provisioned
	}
	want := map[string]bool{}
	for _, r := range requested {
		want[r] = true
	}
	out := []string{}
	for _, svc := range provisioned {
		if want[svc] {
			out = append(out, svc)
		}
	}
	return out
}

func runBackup(ctx context.Context, svc, appName, filePath string) error {
	switch svc {
	case "postgres":
		return postgres.Backup(ctx, appName, filePath)
	case "redis":
		return redis.Backup(ctx, appName, filePath)
	case "rabbitmq":
		return rabbitmq.Backup(ctx, appName, filePath)
	case "aistor":
		return aistor.Backup(ctx, appName, filePath)
	case "mongodb":
		return mongodb.Backup(ctx, appName, filePath)
	}
	return fmt.Errorf("%w: %s", ErrNonBackupable, svc)
}

func timestamp() string {
	// 20260517T193045 — sortable, no separators that conflict with file
	// systems. Matches the TS implementation's substring slice.
	s := time.Now().UTC().Format("20060102T150405Z")
	return strings.TrimSuffix(s, "Z")
}

// Small inline sort to avoid pulling sort just for one slice; n is tiny.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
