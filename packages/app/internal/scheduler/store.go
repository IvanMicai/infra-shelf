package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Schedule struct {
	ID             int64
	AppName        string
	Services       []string
	CronExpr       string
	Timezone       string
	RetentionDays  int
	RetentionCount int
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastRunAt      *time.Time
	NextRunAt      *time.Time
	LastStatus     string
	LastMessage    string
}

type ScheduleInput struct {
	AppName        string
	Services       []string
	CronExpr       string
	Timezone       string
	RetentionDays  int
	RetentionCount int
	Enabled        bool
}

type Run struct {
	ID         int64
	ScheduleID *int64
	AppName    string
	Services   []string
	Status     string
	Output     string
	StartedAt  time.Time
	FinishedAt *time.Time
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schedules (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	app_name TEXT NOT NULL,
	services TEXT NOT NULL DEFAULT '',
	cron_expr TEXT NOT NULL,
	timezone TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	last_run_at TEXT,
	next_run_at TEXT,
	last_status TEXT NOT NULL DEFAULT '',
	last_message TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS backup_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	schedule_id INTEGER,
	app_name TEXT NOT NULL,
	services TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	output TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	finished_at TEXT,
	FOREIGN KEY(schedule_id) REFERENCES schedules(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules(enabled);
CREATE INDEX IF NOT EXISTS idx_backup_runs_started_at ON backup_runs(started_at);
`)
	if err != nil {
		return err
	}
	if err := s.addScheduleColumn("retention_days", "INTEGER NOT NULL DEFAULT 30"); err != nil {
		return err
	}
	if err := s.addScheduleColumn("retention_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return err
}

func (s *Store) addScheduleColumn(name, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(schedules)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if columnName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec(`ALTER TABLE schedules ADD COLUMN ` + name + ` ` + definition)
	return err
}

func (s *Store) CreateSchedule(ctx context.Context, input ScheduleInput) (Schedule, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
INSERT INTO schedules (app_name, services, cron_expr, timezone, retention_days, retention_count, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.AppName,
		joinServices(input.Services),
		input.CronExpr,
		input.Timezone,
		input.RetentionDays,
		input.RetentionCount,
		boolInt(input.Enabled),
		now,
		now,
	)
	if err != nil {
		return Schedule{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Schedule{}, err
	}
	return s.GetSchedule(ctx, id)
}

func (s *Store) GetSchedule(ctx context.Context, id int64) (Schedule, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, app_name, services, cron_expr, timezone, retention_days, retention_count, enabled, created_at, updated_at, last_run_at, next_run_at, last_status, last_message
FROM schedules
WHERE id = ?`, id)
	return scanSchedule(row)
}

func (s *Store) ListSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, app_name, services, cron_expr, timezone, retention_days, retention_count, enabled, created_at, updated_at, last_run_at, next_run_at, last_status, last_message
FROM schedules
ORDER BY enabled DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := []Schedule{}
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, rows.Err()
}

func (s *Store) SetScheduleEnabled(ctx context.Context, id int64, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE schedules
SET enabled = ?, updated_at = ?
WHERE id = ?`, boolInt(enabled), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (s *Store) DeleteSchedule(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (s *Store) UpdateNextRun(ctx context.Context, id int64, next time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE schedules
SET next_run_at = ?, updated_at = ?
WHERE id = ?`, next.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) MarkScheduleRun(ctx context.Context, id int64, status, message string, ranAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE schedules
SET last_run_at = ?, last_status = ?, last_message = ?, updated_at = ?
WHERE id = ?`,
		ranAt.UTC().Format(time.RFC3339),
		status,
		message,
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	return err
}

func (s *Store) StartRun(ctx context.Context, scheduleID *int64, appName string, services []string) (int64, error) {
	started := time.Now().UTC().Format(time.RFC3339)
	var id any
	if scheduleID != nil {
		id = *scheduleID
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO backup_runs (schedule_id, app_name, services, status, started_at)
VALUES (?, ?, ?, ?, ?)`,
		id,
		appName,
		joinServices(services),
		"running",
		started,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) FinishRun(ctx context.Context, id int64, status, output string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE backup_runs
SET status = ?, output = ?, finished_at = ?
WHERE id = ?`,
		status,
		output,
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, schedule_id, app_name, services, status, output, started_at, finished_at
FROM backup_runs
ORDER BY started_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type scheduleScanner interface {
	Scan(dest ...any) error
}

func scanSchedule(scanner scheduleScanner) (Schedule, error) {
	var schedule Schedule
	var services string
	var enabled int
	var createdAt, updatedAt string
	var lastRunAt, nextRunAt sql.NullString

	err := scanner.Scan(
		&schedule.ID,
		&schedule.AppName,
		&services,
		&schedule.CronExpr,
		&schedule.Timezone,
		&schedule.RetentionDays,
		&schedule.RetentionCount,
		&enabled,
		&createdAt,
		&updatedAt,
		&lastRunAt,
		&nextRunAt,
		&schedule.LastStatus,
		&schedule.LastMessage,
	)
	if err != nil {
		return Schedule{}, err
	}

	schedule.Services = splitServices(services)
	schedule.Enabled = enabled == 1
	schedule.CreatedAt = parseTime(createdAt)
	schedule.UpdatedAt = parseTime(updatedAt)
	if lastRunAt.Valid {
		t := parseTime(lastRunAt.String)
		schedule.LastRunAt = &t
	}
	if nextRunAt.Valid {
		t := parseTime(nextRunAt.String)
		schedule.NextRunAt = &t
	}
	return schedule, nil
}

func scanRun(scanner scheduleScanner) (Run, error) {
	var run Run
	var scheduleID sql.NullInt64
	var services string
	var startedAt string
	var finishedAt sql.NullString

	err := scanner.Scan(
		&run.ID,
		&scheduleID,
		&run.AppName,
		&services,
		&run.Status,
		&run.Output,
		&startedAt,
		&finishedAt,
	)
	if err != nil {
		return Run{}, err
	}

	if scheduleID.Valid {
		id := scheduleID.Int64
		run.ScheduleID = &id
	}
	run.Services = splitServices(services)
	run.StartedAt = parseTime(startedAt)
	if finishedAt.Valid {
		t := parseTime(finishedAt.String)
		run.FinishedAt = &t
	}
	return run, nil
}

func requireAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func joinServices(services []string) string {
	return strings.Join(services, ",")
}

func splitServices(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	services := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			services = append(services, part)
		}
	}
	return services
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func BriefOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= 300 {
		return output
	}
	return fmt.Sprintf("%s...", output[:300])
}
