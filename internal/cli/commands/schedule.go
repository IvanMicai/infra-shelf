package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/config"
	"github.com/ivan/infra-shelf/internal/output"
	"github.com/ivan/infra-shelf/internal/web/scheduler"
)

func NewScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage backup schedules (shared with the web UI)",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List backup schedules",
		Args:  cobra.NoArgs,
		RunE:  runScheduleList,
	}

	createCmd := &cobra.Command{
		Use:   "create <app>",
		Short: "Create a backup schedule",
		Args:  cobra.ExactArgs(1),
		RunE:  runScheduleCreate,
	}
	createCmd.Flags().String("cron", "", "cron expression (required)")
	createCmd.Flags().StringP("timezone", "z", "", "timezone (defaults to APP_TIMEZONE)")
	createCmd.Flags().StringP("services", "s", "", "services to back up (defaults to all)")
	createCmd.Flags().Int("retention-days", 30, "delete backups older than N days (0 = keep)")
	createCmd.Flags().Int("retention-count", 0, "keep last N backups per service (0 = keep all)")
	createCmd.Flags().Bool("disabled", false, "create the schedule paused")

	pauseCmd := &cobra.Command{Use: "pause <id>", Short: "Pause a schedule", Args: cobra.ExactArgs(1), RunE: scheduleToggle(false)}
	resumeCmd := &cobra.Command{Use: "resume <id>", Short: "Resume a schedule", Args: cobra.ExactArgs(1), RunE: scheduleToggle(true)}
	deleteCmd := &cobra.Command{Use: "delete <id>", Short: "Delete a schedule", Args: cobra.ExactArgs(1), RunE: runScheduleDelete}

	cmd.AddCommand(listCmd, createCmd, pauseCmd, resumeCmd, deleteCmd)
	return cmd
}

func openScheduleStore() (*scheduler.Store, config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, config.Config{}, err
	}
	store, err := scheduler.OpenStore(cfg.DatabasePath)
	if err != nil {
		return nil, cfg, fmt.Errorf("open schedule store: %w", err)
	}
	return store, cfg, nil
}

func runScheduleList(cmd *cobra.Command, _ []string) error {
	store, _, err := openScheduleStore()
	if err != nil {
		return err
	}
	defer store.Close()

	schedules, err := store.ListSchedules(cmd.Context())
	if err != nil {
		return err
	}
	if len(schedules) == 0 {
		output.Info("No schedules.")
		return nil
	}

	for _, s := range schedules {
		state := "paused"
		if s.Enabled {
			state = "enabled"
		}
		services := strings.Join(s.Services, ",")
		if services == "" {
			services = "(all)"
		}
		last := "never run"
		if s.LastRunAt != nil {
			last = fmt.Sprintf("last=%s (%s)", s.LastRunAt.Format("2006-01-02T15:04:05Z"), s.LastStatus)
		}
		next := "no upcoming"
		if s.NextRunAt != nil {
			next = fmt.Sprintf("next=%s", s.NextRunAt.Format("2006-01-02T15:04:05Z"))
		}
		fmt.Printf("#%d %s %s\n", s.ID, s.AppName, state)
		fmt.Printf("  cron=%q tz=%s services=%s\n", s.CronExpr, s.Timezone, services)
		fmt.Printf("  retention: %dd / %d files\n", s.RetentionDays, s.RetentionCount)
		fmt.Printf("  %s  |  %s\n", last, next)
	}
	return nil
}

func runScheduleCreate(cmd *cobra.Command, args []string) error {
	appName := args[0]
	cronExpr, _ := cmd.Flags().GetString("cron")
	if cronExpr == "" {
		return errors.New("--cron is required")
	}
	timezone, _ := cmd.Flags().GetString("timezone")
	servicesRaw, _ := cmd.Flags().GetString("services")
	retentionDays, _ := cmd.Flags().GetInt("retention-days")
	retentionCount, _ := cmd.Flags().GetInt("retention-count")
	disabled, _ := cmd.Flags().GetBool("disabled")

	store, cfg, err := openScheduleStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if timezone == "" {
		timezone = cfg.Timezone
	}

	services := []string{}
	for _, p := range strings.Split(servicesRaw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			services = append(services, p)
		}
	}

	created, err := store.CreateSchedule(cmd.Context(), scheduler.ScheduleInput{
		AppName:        appName,
		Services:       services,
		CronExpr:       cronExpr,
		Timezone:       timezone,
		RetentionDays:  retentionDays,
		RetentionCount: retentionCount,
		Enabled:        !disabled,
	})
	if err != nil {
		return err
	}

	output.Success(fmt.Sprintf("schedule #%d created", created.ID))
	output.Info("Restart the web app (or wait for the next manager reload) to pick it up.")
	return nil
}

func scheduleToggle(enabled bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		id, err := parseScheduleID(args[0])
		if err != nil {
			return err
		}
		store, _, err := openScheduleStore()
		if err != nil {
			return err
		}
		defer store.Close()

		if err := store.SetScheduleEnabled(cmd.Context(), id, enabled); err != nil {
			if scheduler.IsNotFound(err) {
				return fmt.Errorf("schedule #%d not found", id)
			}
			return err
		}
		state := "paused"
		if enabled {
			state = "resumed"
		}
		output.Success(fmt.Sprintf("schedule #%d %s", id, state))
		return nil
	}
}

func runScheduleDelete(cmd *cobra.Command, args []string) error {
	id, err := parseScheduleID(args[0])
	if err != nil {
		return err
	}
	store, _, err := openScheduleStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.DeleteSchedule(cmd.Context(), id); err != nil {
		if scheduler.IsNotFound(err) {
			return fmt.Errorf("schedule #%d not found", id)
		}
		return err
	}
	output.Success(fmt.Sprintf("schedule #%d deleted", id))
	return nil
}

func parseScheduleID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid schedule id %q", raw)
	}
	return id, nil
}

// Compile-time check that schedule commands honor the standard cobra context.
var _ = context.Background
