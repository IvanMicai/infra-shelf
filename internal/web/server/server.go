package server

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/IvanMicai/infra-shelf/internal/backup"
	"github.com/IvanMicai/infra-shelf/internal/config"
	"github.com/IvanMicai/infra-shelf/internal/docker"
	"github.com/IvanMicai/infra-shelf/internal/envspec"
	"github.com/IvanMicai/infra-shelf/internal/registry"
	"github.com/IvanMicai/infra-shelf/internal/shelfcore"
	"github.com/IvanMicai/infra-shelf/internal/web/assets"
	"github.com/IvanMicai/infra-shelf/internal/web/auth"
	"github.com/IvanMicai/infra-shelf/internal/web/backupservice"
	"github.com/IvanMicai/infra-shelf/internal/web/runlog"
	"github.com/IvanMicai/infra-shelf/internal/web/scheduler"
)

type Server struct {
	cfg       config.Config
	registry  *registry.Store
	backups   *backupservice.Service
	store     *scheduler.Store
	manager   *scheduler.Manager
	templates *template.Template
	logger    *log.Logger
}

type PageData struct {
	Title         string
	Active        string
	Flash         Flash
	Config        config.Config
	Apps          []registry.App
	App           registry.App
	EnvBlocks     []registry.EnvBlock
	EnvFile       string
	Backups       []backup.File
	Statuses      []docker.Status
	Schedules     []scheduler.Schedule
	Runs          []scheduler.Run
	SelectedApp   string
	SelectedFile  string
	BackupCount   int
	ScheduleCount int
	S3Enabled     bool
	S3Destination string
}

type Flash struct {
	Kind    string
	Message string
}

func New(cfg config.Config, backups *backupservice.Service, store *scheduler.Store, manager *scheduler.Manager, logger *log.Logger) (*Server, error) {
	funcs := template.FuncMap{
		"formatTime":       formatTime,
		"formatBytes":      formatBytes,
		"join":             strings.Join,
		"serviceLabel":     serviceLabel,
		"statusClass":      statusClass,
		"displayServices":  scheduler.DisplayServices,
		"displayTarget":    scheduler.DisplayTarget,
		"displayRetention": displayRetention,
	}

	templates, err := template.New("").Funcs(funcs).ParseFS(assets.Files, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:       cfg,
		registry:  registry.NewStore(cfg.RegistryPath),
		backups:   backups,
		store:     store,
		manager:   manager,
		templates: templates,
		logger:    logger,
	}, nil
}

// newEngine creates a shelfcore.Engine wired to a fresh runlog buffer so
// handlers can surface captured output back to the user on error.
func (s *Server) newEngine() (*shelfcore.Engine, *runlog.Buffer) {
	rep := runlog.New()
	return shelfcore.New(s.registry, s.cfg.BackupsDir, rep), rep
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(assets.Files, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /", s.dashboard)
	mux.HandleFunc("POST /infra/start", s.startInfrastructure)
	mux.HandleFunc("GET /apps", s.appsPage)
	mux.HandleFunc("POST /apps", s.createApp)
	mux.HandleFunc("GET /apps/{name}", s.appDetail)
	mux.HandleFunc("GET /apps/{name}/credentials", s.appCredentials)
	mux.HandleFunc("GET /apps/{name}/env", s.downloadEnv)
	mux.HandleFunc("POST /apps/{name}/backup", s.backupApp)
	mux.HandleFunc("POST /apps/{name}/services", s.addServices)
	mux.HandleFunc("POST /apps/{name}/addons/{addon}/detach", s.detachAddon)
	mux.HandleFunc("POST /apps/{name}/remove", s.removeApp)

	mux.HandleFunc("GET /backups", s.backupsPage)
	mux.HandleFunc("POST /backups/run", s.backupAll)
	mux.HandleFunc("POST /backups/s3/sync", s.syncBackupsToS3)
	mux.HandleFunc("GET /backups/{app}/{file}/download", s.downloadBackup)
	mux.HandleFunc("POST /backups/{app}/{file}/restore", s.restoreBackup)
	mux.HandleFunc("POST /backups/{app}/{file}/delete", s.deleteBackup)

	mux.HandleFunc("GET /schedules", s.schedulesPage)
	mux.HandleFunc("POST /schedules", s.createSchedule)
	mux.HandleFunc("POST /schedules/{id}/pause", s.pauseSchedule)
	mux.HandleFunc("POST /schedules/{id}/resume", s.resumeSchedule)
	mux.HandleFunc("POST /schedules/{id}/delete", s.deleteSchedule)
	mux.HandleFunc("POST /schedules/{id}/run", s.runSchedule)

	mux.HandleFunc("GET /fragments/status", s.statusFragment)

	mux.HandleFunc("GET /logout", s.logout)

	return auth.Basic(s.cfg.Username, s.cfg.Password, secureHeaders(mux))
}

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="infra-shelf-logout"`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Logged out - infra-shelf</title>
<link rel="stylesheet" href="/static/app.css"></head>
<body><main class="content"><section class="panel">
<h1>Logged out</h1>
<p>Voce saiu da sessao do infra-shelf.</p>
<p><a class="button primary" href="/">Sign in again</a></p>
</section></main></body></html>`))
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	apps, err := s.registry.ListApps()
	if err != nil {
		s.renderError(w, err)
		return
	}
	backups, err := backup.List(s.cfg.BackupsDir)
	if err != nil {
		s.renderError(w, err)
		return
	}
	schedules, err := s.store.ListSchedules(r.Context())
	if err != nil {
		s.renderError(w, err)
		return
	}
	runs, err := s.store.ListRuns(r.Context(), 8)
	if err != nil {
		s.renderError(w, err)
		return
	}

	data := s.page(r, "Dashboard", "dashboard")
	data.Apps = apps
	data.Statuses = docker.ListStatus(r.Context())
	data.Backups = firstBackups(backups, 8)
	data.Runs = runs
	data.BackupCount = len(backups)
	data.ScheduleCount = len(schedules)
	s.render(w, "dashboard.html", data)
}

func (s *Server) appsPage(w http.ResponseWriter, r *http.Request) {
	apps, err := s.registry.ListApps()
	if err != nil {
		s.renderError(w, err)
		return
	}
	data := s.page(r, "Apps", "apps")
	data.Apps = apps
	data.Statuses = docker.ListStatus(r.Context())
	s.render(w, "apps.html", data)
}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirect(w, r, "/apps", "error", err.Error())
		return
	}

	appName := strings.TrimSpace(r.FormValue("app_name"))
	services, err := registry.ParseServices(r.Form["services"])
	if err != nil {
		s.redirect(w, r, "/apps", "error", err.Error())
		return
	}
	if err := registry.ValidateAppName(appName); err != nil {
		s.redirect(w, r, "/apps", "error", "invalid app name")
		return
	}
	if len(services) == 0 {
		s.redirect(w, r, "/apps", "error", "select at least one service")
		return
	}
	envOpts, err := parseEnvsField(r.FormValue("envs"))
	if err != nil {
		s.redirect(w, r, "/apps", "error", err.Error())
		return
	}

	engine, buf := s.newEngine()
	if _, err := engine.SetupApp(r.Context(), appName, shelfcore.SetupOptions{
		Services: services,
		Envs:     envOpts.Envs,
		Env:      envOpts.Env,
	}); err != nil {
		s.redirect(w, r, "/apps", "error", withOutput(err, buf.String()))
		return
	}

	// Multi-env expansion lands on the first sibling; otherwise on the app
	// name itself.
	landing := appName
	if len(envOpts.Envs) > 0 {
		landing = appName + "-" + envOpts.Envs[0]
	}
	s.redirect(w, r, "/apps/"+url.PathEscape(landing), "success", "app provisioned")
}

func (s *Server) startInfrastructure(w http.ResponseWriter, r *http.Request) {
	target := safeRedirectTarget(r.Referer())
	cmd := exec.CommandContext(r.Context(), "docker", "compose", "--env-file", ".env", "up", "-d")
	cmd.Dir = s.cfg.RootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.redirect(w, r, target, "error", withOutput(err, strings.TrimSpace(string(output))))
		return
	}
	s.redirect(w, r, target, "success", "infrastructure started")
}

func (s *Server) appDetail(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	app, ok, err := s.registry.GetApp(appName)
	if err != nil {
		s.renderError(w, err)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	backups, err := backup.ListForApp(s.cfg.BackupsDir, appName)
	if err != nil {
		s.renderError(w, err)
		return
	}

	data := s.page(r, appName, "apps")
	data.App = app
	data.Backups = backups
	s.render(w, "app_detail.html", data)
}

func (s *Server) appCredentials(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	app, ok, err := s.registry.GetApp(appName)
	if err != nil {
		s.renderError(w, err)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	if service := r.URL.Query().Get("service"); service != "" {
		for _, info := range app.ServiceInfos() {
			if info.Name == service {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w,
					`<pre id="cred-%s" class="env-box">%s</pre>`,
					template.HTMLEscapeString(info.Name),
					template.HTMLEscapeString(info.EnvBody),
				)
				return
			}
		}
		http.NotFound(w, r)
		return
	}

	data := s.page(r, appName, "apps")
	data.App = app
	data.EnvFile = app.EnvFile()
	s.render(w, "credentials.html", data)
}

func (s *Server) backupApp(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	if err := r.ParseForm(); err != nil {
		s.redirect(w, r, "/apps/"+url.PathEscape(appName), "error", err.Error())
		return
	}
	services, err := registry.ParseServices(r.Form["services"])
	if err != nil {
		s.redirect(w, r, "/apps/"+url.PathEscape(appName), "error", err.Error())
		return
	}

	result, err := s.runManualBackup(r.Context(), appName, false, services)
	if err != nil {
		s.redirect(w, r, "/apps/"+url.PathEscape(appName), "error", withOutput(err, result.Log))
		return
	}
	s.redirect(w, r, "/apps/"+url.PathEscape(appName), "success", "backup completed")
}

func (s *Server) downloadEnv(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	app, ok, err := s.registry.GetApp(appName)
	if err != nil {
		s.renderError(w, err)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.env"`, appName))
	_, _ = w.Write([]byte(app.EnvFile() + "\n"))
}

func (s *Server) addServices(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	target := "/apps/" + url.PathEscape(appName)

	if _, ok, err := s.registry.GetApp(appName); err != nil {
		s.redirect(w, r, target, "error", err.Error())
		return
	} else if !ok {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirect(w, r, target, "error", err.Error())
		return
	}
	services, err := registry.ParseServices(r.Form["services"])
	if err != nil {
		s.redirect(w, r, target, "error", err.Error())
		return
	}
	if len(services) == 0 {
		s.redirect(w, r, target, "error", "select at least one service")
		return
	}

	engine, buf := s.newEngine()
	if _, err := engine.AddServices(r.Context(), appName, shelfcore.AddOptions{Services: services}); err != nil {
		s.redirect(w, r, target, "error", withOutput(err, buf.String()))
		return
	}
	s.redirect(w, r, target, "success", "services attached")
}

// detachAddon strips an addon (e.g. signoz) from the app's registry entry.
// Addons own no per-app resources, so this is purely a config flip.
func (s *Server) detachAddon(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	addon := r.PathValue("addon")
	target := "/apps/" + url.PathEscape(appName)

	if _, ok := registry.ValidServices[addon]; !ok {
		s.redirect(w, r, target, "error", "invalid addon")
		return
	}

	engine, buf := s.newEngine()
	if err := engine.DetachServices(r.Context(), appName, []string{addon}); err != nil {
		s.redirect(w, r, target, "error", withOutput(err, buf.String()))
		return
	}
	s.redirect(w, r, target, "success", addon+" detached")
}

func (s *Server) removeApp(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	engine, buf := s.newEngine()
	if err := engine.RemoveApp(r.Context(), appName); err != nil {
		s.redirect(w, r, "/apps/"+url.PathEscape(appName), "error", withOutput(err, buf.String()))
		return
	}
	s.redirect(w, r, "/apps", "success", "app removed")
}

func (s *Server) backupsPage(w http.ResponseWriter, r *http.Request) {
	apps, err := s.registry.ListApps()
	if err != nil {
		s.renderError(w, err)
		return
	}
	backups, err := backup.List(s.cfg.BackupsDir)
	if err != nil {
		s.renderError(w, err)
		return
	}
	data := s.page(r, "Backups", "backups")
	data.Apps = apps
	data.Backups = backups
	s.render(w, "backups.html", data)
}

func (s *Server) backupAll(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirect(w, r, "/backups", "error", err.Error())
		return
	}
	services, err := registry.ParseServices(r.Form["services"])
	if err != nil {
		s.redirect(w, r, "/backups", "error", err.Error())
		return
	}
	result, err := s.runManualBackup(r.Context(), "*", true, services)
	if err != nil {
		s.redirect(w, r, "/backups", "error", withOutput(err, result.Log))
		return
	}
	s.redirect(w, r, "/backups", "success", "backup completed")
}

func (s *Server) syncBackupsToS3(w http.ResponseWriter, r *http.Request) {
	uploaded, err := s.backups.UploadAll(r.Context())
	if err != nil {
		s.redirect(w, r, "/backups", "error", err.Error())
		return
	}
	s.redirect(w, r, "/backups", "success", fmt.Sprintf("uploaded %d backup file(s) to S3", len(uploaded)))
}

func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	path, err := backup.Resolve(s.cfg.BackupsDir, r.PathValue("app"), r.PathValue("file"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	http.ServeFile(w, r, path)
}

func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("app")
	fileName := r.PathValue("file")
	path, err := backup.Resolve(s.cfg.BackupsDir, appName, fileName)
	if err != nil {
		s.redirect(w, r, "/backups", "error", err.Error())
		return
	}

	engine, buf := s.newEngine()
	target := "/apps/" + url.PathEscape(appName)
	if err := engine.RestoreFromFile(r.Context(), appName, path); err != nil {
		s.redirect(w, r, target, "error", withOutput(err, buf.String()))
		return
	}
	s.redirect(w, r, target, "success", "backup restored")
}

func (s *Server) deleteBackup(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("app")
	fileName := r.PathValue("file")
	redirect := r.Referer()
	if redirect == "" {
		redirect = "/backups"
	}
	if _, err := s.backups.DeleteFile(r.Context(), appName, fileName); err != nil {
		s.redirect(w, r, redirect, "error", err.Error())
		return
	}
	s.redirect(w, r, redirect, "success", fmt.Sprintf("deleted %s", fileName))
}

func (s *Server) schedulesPage(w http.ResponseWriter, r *http.Request) {
	apps, err := s.registry.ListApps()
	if err != nil {
		s.renderError(w, err)
		return
	}
	schedules, err := s.store.ListSchedules(r.Context())
	if err != nil {
		s.renderError(w, err)
		return
	}
	runs, err := s.store.ListRuns(r.Context(), 20)
	if err != nil {
		s.renderError(w, err)
		return
	}

	data := s.page(r, "Schedules", "schedules")
	data.Apps = apps
	data.Schedules = schedules
	data.Runs = runs
	s.render(w, "schedules.html", data)
}

func (s *Server) createSchedule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}

	appName := strings.TrimSpace(r.FormValue("app_name"))
	if appName == "" {
		appName = "*"
	}
	if appName != "*" {
		if err := registry.ValidateAppName(appName); err != nil {
			s.redirect(w, r, "/schedules", "error", "invalid app name")
			return
		}
		if _, ok, err := s.registry.GetApp(appName); err != nil {
			s.redirect(w, r, "/schedules", "error", err.Error())
			return
		} else if !ok {
			s.redirect(w, r, "/schedules", "error", "app not found")
			return
		}
	}

	services, err := registry.ParseServices(r.Form["services"])
	if err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}

	cronExpr := strings.TrimSpace(r.FormValue("cron_expr"))
	if cronExpr == "" {
		s.redirect(w, r, "/schedules", "error", "cron expression is required")
		return
	}
	if err := s.manager.Validate(cronExpr); err != nil {
		s.redirect(w, r, "/schedules", "error", "invalid cron expression")
		return
	}

	timezone := strings.TrimSpace(r.FormValue("timezone"))
	if timezone == "" {
		timezone = s.cfg.Timezone
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		s.redirect(w, r, "/schedules", "error", "invalid timezone")
		return
	}

	_, err = s.store.CreateSchedule(r.Context(), scheduler.ScheduleInput{
		AppName:        appName,
		Services:       services,
		CronExpr:       cronExpr,
		Timezone:       timezone,
		RetentionDays:  parseNonNegativeInt(r.FormValue("retention_days"), 30),
		RetentionCount: parseNonNegativeInt(r.FormValue("retention_count"), 0),
		Enabled:        true,
	})
	if err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	if err := s.manager.Reload(r.Context()); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	s.redirect(w, r, "/schedules", "success", "schedule created")
}

func (s *Server) pauseSchedule(w http.ResponseWriter, r *http.Request) {
	s.setScheduleEnabled(w, r, false)
}

func (s *Server) resumeSchedule(w http.ResponseWriter, r *http.Request) {
	s.setScheduleEnabled(w, r, true)
}

func (s *Server) setScheduleEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id, err := pathID(r)
	if err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	if err := s.store.SetScheduleEnabled(r.Context(), id, enabled); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	if err := s.manager.Reload(r.Context()); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	s.redirect(w, r, "/schedules", "success", "schedule updated")
}

func (s *Server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	if err := s.store.DeleteSchedule(r.Context(), id); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	if err := s.manager.Reload(r.Context()); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	s.redirect(w, r, "/schedules", "success", "schedule deleted")
}

func (s *Server) runSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	if err := s.manager.RunNow(r.Context(), id); err != nil {
		s.redirect(w, r, "/schedules", "error", err.Error())
		return
	}
	s.redirect(w, r, "/schedules", "success", "schedule started")
}

func (s *Server) statusFragment(w http.ResponseWriter, r *http.Request) {
	data := s.page(r, "Status", "dashboard")
	data.Statuses = docker.ListStatus(r.Context())
	s.render(w, "status_grid.html", data)
}

func (s *Server) runManualBackup(ctx context.Context, appName string, all bool, services []string) (backupservice.Result, error) {
	recordName := appName
	if all {
		recordName = "*"
	}
	runID, err := s.store.StartRun(ctx, nil, recordName, services)
	if err != nil {
		return backupservice.Result{}, err
	}

	result, runErr := s.backups.Backup(ctx, appName, all, services)
	status := "success"
	output := result.Log
	if runErr != nil {
		status = "failed"
		if output != "" {
			output += "\n"
		}
		output += runErr.Error()
	}
	if err := s.store.FinishRun(ctx, runID, status, strings.TrimSpace(output)); err != nil && runErr == nil {
		return result, err
	}
	return result, runErr
}

func (s *Server) page(r *http.Request, title, active string) PageData {
	return PageData{
		Title:         title,
		Active:        active,
		Flash:         flashFromQuery(r),
		Config:        s.cfg,
		S3Enabled:     s.backups.S3Enabled(),
		S3Destination: s.backups.S3Destination(),
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Printf("render %s: %v", name, err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, err error) {
	s.logger.Printf("request failed: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, target, kind, message string) {
	if message != "" {
		values := url.Values{}
		values.Set(kind, message)
		separator := "?"
		if strings.Contains(target, "?") {
			separator = "&"
		}
		target += separator + values.Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func safeRedirectTarget(referer string) string {
	if referer == "" {
		return "/"
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Path == "" || parsed.Path[0] != '/' {
		return "/"
	}
	target := parsed.Path
	if parsed.RawQuery != "" {
		target += "?" + parsed.RawQuery
	}
	return target
}

func flashFromQuery(r *http.Request) Flash {
	if value := r.URL.Query().Get("success"); value != "" {
		return Flash{Kind: "success", Message: value}
	}
	if value := r.URL.Query().Get("error"); value != "" {
		return Flash{Kind: "error", Message: value}
	}
	return Flash{}
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func parseNonNegativeInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func withOutput(err error, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return err.Error()
	}
	return err.Error() + ": " + scheduler.BriefOutput(output)
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func firstBackups(files []backup.File, limit int) []backup.File {
	if len(files) <= limit {
		return files
	}
	return files[:limit]
}

func formatTime(value any) string {
	var t time.Time
	switch v := value.(type) {
	case time.Time:
		t = v
	case *time.Time:
		if v == nil {
			return "-"
		}
		t = *v
	default:
		return "-"
	}
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func serviceLabel(service string) string {
	switch service {
	case "postgres":
		return "PostgreSQL"
	case "redis":
		return "Redis"
	case "rabbitmq":
		return "RabbitMQ"
	case "aistor":
		return "AIStor"
	case "mongodb":
		return "MongoDB"
	case "signoz":
		return "SignOz"
	default:
		return service
	}
}

// parseEnvsField turns the form's Environment(s) input into an envspec.Options
// pair. A single value tags one app; a CSV expands into siblings.
func parseEnvsField(raw string) (envspec.Options, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return envspec.Options{}, nil
	}
	if !strings.Contains(raw, ",") {
		env, err := envspec.ParseSingleEnv(raw)
		if err != nil {
			return envspec.Options{}, err
		}
		if env == "" {
			return envspec.Options{}, nil
		}
		return envspec.Options{Env: env}, nil
	}
	envs, err := envspec.ParseEnvs(raw)
	if err != nil {
		return envspec.Options{}, err
	}
	return envspec.Options{Envs: envs}, nil
}

func statusClass(status string) string {
	switch status {
	case "running":
		return "running"
	case "exited", "dead":
		return "stopped"
	case "":
		return "missing"
	default:
		return "warning"
	}
}

func displayRetention(days, count int) string {
	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if count > 0 {
		parts = append(parts, fmt.Sprintf("%d files", count))
	}
	if len(parts) == 0 {
		return "keep forever"
	}
	return strings.Join(parts, " / ")
}

// errors below kept for compile-time guarantee that the sentinels are wired.
var _ = errors.Is
