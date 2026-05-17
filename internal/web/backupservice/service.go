// Package backupservice orchestrates backup runs initiated by the web UI or
// scheduler: invokes the shelfcore library directly (no subprocess), mirrors
// the produced files to S3, and applies retention policies. The captured run
// log is returned so handlers can persist it in the backup_runs table.
package backupservice

import (
	"context"
	"fmt"
	"log"

	"github.com/ivan/infra-shelf/internal/backup"
	"github.com/ivan/infra-shelf/internal/s3backup"
	"github.com/ivan/infra-shelf/internal/shelfcore"
	"github.com/ivan/infra-shelf/internal/web/runlog"
)

// Result bundles everything a caller needs after a backup run: a flattened
// list of produced files (whether or not they were uploaded), the captured
// progress log for storage, and the underlying error if any service failed.
type Result struct {
	Files []shelfcore.BackupFile
	Log   string
}

type Service struct {
	engineFactory func(reporter shelfcore.Reporter) *shelfcore.Engine
	backupsDir    string
	s3            *s3backup.Client
	logger        *log.Logger
}

// EngineFactory is invoked once per Backup call with a freshly-allocated
// runlog reporter. Letting the caller construct the engine keeps the registry
// store reuse policy in the caller's hands (it's the same store the server
// already opened).
type EngineFactory = func(reporter shelfcore.Reporter) *shelfcore.Engine

func New(factory EngineFactory, backupsDir string, s3 *s3backup.Client, logger *log.Logger) *Service {
	return &Service{
		engineFactory: factory,
		backupsDir:    backupsDir,
		s3:            s3,
		logger:        logger,
	}
}

// Backup runs a one-shot backup with no retention. Equivalent to
// BackupWithRetention(..., PruneOptions{}).
func (s *Service) Backup(ctx context.Context, appName string, all bool, services []string) (Result, error) {
	return s.BackupWithRetention(ctx, appName, all, services, backup.PruneOptions{})
}

// BackupWithRetention runs the backup, uploads new files to S3 (if enabled),
// then applies the supplied retention policy.
func (s *Service) BackupWithRetention(ctx context.Context, appName string, all bool, services []string, retention backup.PruneOptions) (Result, error) {
	rep := runlog.New()
	engine := s.engineFactory(rep)

	var (
		files []shelfcore.BackupFile
		err   error
	)
	if all {
		files, err = engine.BackupAll(ctx, shelfcore.BackupOptions{Services: services})
	} else {
		files, err = engine.BackupApp(ctx, appName, shelfcore.BackupOptions{Services: services})
	}
	result := Result{Files: files}
	if err != nil {
		rep.AppendLine(err.Error())
		result.Log = rep.String()
		return result, err
	}

	if s.s3 != nil && s.s3.Enabled() && len(files) > 0 {
		toUpload := convertFiles(files)
		uploaded, uerr := s.s3.UploadMany(ctx, toUpload)
		if uerr != nil {
			rep.AppendLine(uerr.Error())
			result.Log = rep.String()
			return result, uerr
		}
		msg := fmt.Sprintf("Uploaded %d backup file(s) to %s", len(uploaded), s.s3.Destination())
		rep.AppendLine(msg)
		if s.logger != nil {
			s.logger.Print(msg)
		}
	}

	if perr := s.applyRetention(ctx, rep, retention); perr != nil {
		result.Log = rep.String()
		return result, perr
	}

	result.Log = rep.String()
	return result, nil
}

// UploadAll mirrors every existing backup file to S3, ignoring whether it was
// produced by this process. Useful as a one-off catch-up button.
func (s *Service) UploadAll(ctx context.Context) ([]s3backup.UploadedFile, error) {
	if s.s3 == nil || !s.s3.Enabled() {
		return nil, fmt.Errorf("S3 backup is not configured")
	}
	files, err := backup.List(s.backupsDir)
	if err != nil {
		return nil, err
	}
	return s.s3.UploadMany(ctx, files)
}

// DeleteFile removes a local backup file (and its S3 mirror when enabled).
func (s *Service) DeleteFile(ctx context.Context, app, name string) (backup.File, error) {
	file, err := backup.Delete(s.backupsDir, app, name)
	if err != nil {
		return backup.File{}, err
	}
	if s.s3 != nil && s.s3.Enabled() {
		if err := s.s3.DeleteMany(ctx, []backup.File{file}); err != nil {
			return file, err
		}
	}
	return file, nil
}

func (s *Service) S3Enabled() bool {
	return s.s3 != nil && s.s3.Enabled()
}

func (s *Service) S3Destination() string {
	if s.s3 == nil {
		return "not configured"
	}
	return s.s3.Destination()
}

func (s *Service) applyRetention(ctx context.Context, rep *runlog.Buffer, retention backup.PruneOptions) error {
	deleted, err := backup.Prune(s.backupsDir, retention)
	if err != nil {
		rep.AppendLine(err.Error())
		return err
	}
	if len(deleted) == 0 {
		return nil
	}
	if s.s3 != nil && s.s3.Enabled() {
		if err := s.s3.DeleteMany(ctx, deleted); err != nil {
			rep.AppendLine(err.Error())
			return err
		}
	}
	msg := fmt.Sprintf("Pruned %d backup file(s)", len(deleted))
	rep.AppendLine(msg)
	if s.logger != nil {
		s.logger.Print(msg)
	}
	return nil
}

// convertFiles maps shelfcore-produced files into the backup.File shape the
// s3backup client expects. The two types intentionally don't share a package
// to keep s3backup decoupled from shelfcore.
func convertFiles(files []shelfcore.BackupFile) []backup.File {
	out := make([]backup.File, 0, len(files))
	for _, f := range files {
		out = append(out, backup.File{
			App:     f.App,
			Service: f.Service,
			Path:    f.Path,
			Name:    pathBase(f.Path),
		})
	}
	return out
}

func pathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
