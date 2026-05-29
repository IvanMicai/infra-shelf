package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/IvanMicai/infra-shelf/internal/backup"
	"github.com/IvanMicai/infra-shelf/internal/web/backupservice"
	"github.com/robfig/cron/v3"
)

// BackupRunner runs a backup with optional retention and returns the captured
// run log + any error. The web's backupservice.Service satisfies this; tests
// can fake it without spinning up shelfcore.
type BackupRunner interface {
	BackupWithRetention(ctx context.Context, appName string, all bool, services []string, retention backup.PruneOptions) (backupservice.Result, error)
}

type Manager struct {
	store     *Store
	runner    BackupRunner
	cron      *cron.Cron
	entries   map[int64]cron.EntryID
	location  *time.Location
	parser    cron.Parser
	logger    *log.Logger
	entryLock sync.Mutex
}

func NewManager(store *Store, runner BackupRunner, timezone string, logger *log.Logger) (*Manager, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		store:    store,
		runner:   runner,
		location: location,
		logger:   logger,
		entries:  map[int64]cron.EntryID{},
		parser: cron.NewParser(
			cron.Minute |
				cron.Hour |
				cron.Dom |
				cron.Month |
				cron.Dow |
				cron.Descriptor,
		),
	}

	manager.cron = cron.New(
		cron.WithLocation(location),
		cron.WithParser(manager.parser),
		cron.WithLogger(cron.PrintfLogger(logger)),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.PrintfLogger(logger)),
			cron.Recover(cron.PrintfLogger(logger)),
		),
	)

	return manager, nil
}

func (m *Manager) Start() {
	m.cron.Start()
}

func (m *Manager) Stop(ctx context.Context) {
	stopCtx := m.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
	}
}

func (m *Manager) Reload(ctx context.Context) error {
	m.entryLock.Lock()
	defer m.entryLock.Unlock()

	for _, entryID := range m.entries {
		m.cron.Remove(entryID)
	}
	m.entries = map[int64]cron.EntryID{}

	schedules, err := m.store.ListSchedules(ctx)
	if err != nil {
		return err
	}

	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		if err := m.addSchedule(ctx, schedule); err != nil {
			m.logger.Printf("schedule %d skipped: %v", schedule.ID, err)
		}
	}

	return nil
}

func (m *Manager) Validate(expr string) error {
	_, err := m.parser.Parse(expr)
	return err
}

func (m *Manager) addSchedule(ctx context.Context, schedule Schedule) error {
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return err
	}

	spec := schedule.CronExpr
	if location.String() != m.location.String() {
		spec = "CRON_TZ=" + location.String() + " " + schedule.CronExpr
	}

	entryID, err := m.cron.AddFunc(spec, func() {
		m.runSchedule(schedule.ID)
	})
	if err != nil {
		return err
	}
	m.entries[schedule.ID] = entryID

	parsed, err := m.parser.Parse(schedule.CronExpr)
	if err == nil {
		next := parsed.Next(time.Now().In(location))
		_ = m.store.UpdateNextRun(ctx, schedule.ID, next)
	}

	return nil
}

func (m *Manager) RunNow(ctx context.Context, scheduleID int64) error {
	schedule, err := m.store.GetSchedule(ctx, scheduleID)
	if err != nil {
		return err
	}
	go m.run(schedule)
	return nil
}

func (m *Manager) runSchedule(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	schedule, err := m.store.GetSchedule(ctx, id)
	if err != nil {
		m.logger.Printf("load schedule %d: %v", id, err)
		return
	}
	m.run(schedule)
}

func (m *Manager) run(schedule Schedule) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	runID, err := m.store.StartRun(ctx, &schedule.ID, schedule.AppName, schedule.Services)
	if err != nil {
		m.logger.Printf("record schedule %d run: %v", schedule.ID, err)
		return
	}

	all := schedule.AppName == "*"
	appName := schedule.AppName
	if all {
		appName = ""
	}

	result, runErr := m.runner.BackupWithRetention(ctx, appName, all, schedule.Services, backup.PruneOptions{
		AppName:   schedule.AppName,
		All:       all,
		Services:  schedule.Services,
		KeepDays:  schedule.RetentionDays,
		KeepCount: schedule.RetentionCount,
	})

	status := "success"
	output := result.Log
	if runErr != nil {
		status = "failed"
		if output != "" {
			output += "\n"
		}
		output += runErr.Error()
	}

	if err := m.store.FinishRun(ctx, runID, status, strings.TrimSpace(output)); err != nil {
		m.logger.Printf("finish schedule %d run: %v", schedule.ID, err)
	}
	if err := m.store.MarkScheduleRun(ctx, schedule.ID, status, BriefOutput(output), time.Now()); err != nil {
		m.logger.Printf("mark schedule %d run: %v", schedule.ID, err)
	}

	if runErr != nil {
		m.logger.Printf("schedule %d failed: %v", schedule.ID, runErr)
		return
	}
	m.logger.Printf("schedule %d completed", schedule.ID)
}

func DisplayTarget(appName string) string {
	if appName == "*" {
		return "all apps"
	}
	return appName
}

func DisplayServices(services []string) string {
	if len(services) == 0 {
		return "all services"
	}
	return strings.Join(services, ", ")
}

func Describe(schedule Schedule) string {
	return fmt.Sprintf("%s / %s / %s", DisplayTarget(schedule.AppName), DisplayServices(schedule.Services), schedule.CronExpr)
}
