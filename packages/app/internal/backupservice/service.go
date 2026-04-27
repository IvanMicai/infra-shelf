package backupservice

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ivan/infra-shelf/packages/app/internal/backup"
	"github.com/ivan/infra-shelf/packages/app/internal/runner"
	"github.com/ivan/infra-shelf/packages/app/internal/s3backup"
)

type Service struct {
	cli        *runner.CLI
	backupsDir string
	s3         *s3backup.Client
	logger     *log.Logger
}

func New(cli *runner.CLI, backupsDir string, s3 *s3backup.Client, logger *log.Logger) *Service {
	return &Service{
		cli:        cli,
		backupsDir: backupsDir,
		s3:         s3,
		logger:     logger,
	}
}

func (s *Service) Backup(ctx context.Context, appName string, all bool, services []string) (runner.Result, error) {
	return s.BackupWithRetention(ctx, appName, all, services, backup.PruneOptions{})
}

func (s *Service) BackupWithRetention(ctx context.Context, appName string, all bool, services []string, retention backup.PruneOptions) (runner.Result, error) {
	before, err := backup.TakeSnapshot(s.backupsDir)
	if err != nil {
		return runner.Result{}, err
	}

	result, runErr := s.cli.Backup(ctx, appName, all, services)
	if runErr != nil {
		return result, runErr
	}

	newFiles, err := s.newBackupFiles(before)
	if err != nil {
		return result, err
	}

	if s.s3 == nil || !s.s3.Enabled() || len(newFiles) == 0 {
		return s.applyRetention(ctx, result, retention)
	}

	uploaded, err := s.s3.UploadMany(ctx, newFiles)
	if err != nil {
		result.Output = appendLine(result.Output, err.Error())
		return result, err
	}

	message := fmt.Sprintf("Uploaded %d backup file(s) to %s", len(uploaded), s.s3.Destination())
	result.Output = appendLine(result.Output, message)
	if s.logger != nil {
		s.logger.Print(message)
	}
	return s.applyRetention(ctx, result, retention)
}

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

func (s *Service) S3Enabled() bool {
	return s.s3 != nil && s.s3.Enabled()
}

func (s *Service) S3Destination() string {
	if s.s3 == nil {
		return "not configured"
	}
	return s.s3.Destination()
}

func (s *Service) newBackupFiles(before backup.Snapshot) ([]backup.File, error) {
	after, err := backup.List(s.backupsDir)
	if err != nil {
		return nil, err
	}
	return backup.Diff(before, after), nil
}

func (s *Service) applyRetention(ctx context.Context, result runner.Result, retention backup.PruneOptions) (runner.Result, error) {
	deleted, err := backup.Prune(s.backupsDir, retention)
	if err != nil {
		result.Output = appendLine(result.Output, err.Error())
		return result, err
	}
	if len(deleted) == 0 {
		return result, nil
	}

	if s.s3 != nil && s.s3.Enabled() {
		if err := s.s3.DeleteMany(ctx, deleted); err != nil {
			result.Output = appendLine(result.Output, err.Error())
			return result, err
		}
	}

	message := fmt.Sprintf("Pruned %d backup file(s)", len(deleted))
	result.Output = appendLine(result.Output, message)
	if s.logger != nil {
		s.logger.Print(message)
	}
	return result, nil
}

func appendLine(output, line string) string {
	output = strings.TrimSpace(output)
	line = strings.TrimSpace(line)
	if output == "" {
		return line
	}
	if line == "" {
		return output
	}
	return output + "\n" + line
}
