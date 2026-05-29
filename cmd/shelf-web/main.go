package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IvanMicai/infra-shelf/internal/config"
	"github.com/IvanMicai/infra-shelf/internal/registry"
	"github.com/IvanMicai/infra-shelf/internal/s3backup"
	"github.com/IvanMicai/infra-shelf/internal/shelfcore"
	"github.com/IvanMicai/infra-shelf/internal/web/backupservice"
	"github.com/IvanMicai/infra-shelf/internal/web/scheduler"
	"github.com/IvanMicai/infra-shelf/internal/web/server"
)

func main() {
	logger := log.New(os.Stdout, "infra-shelf-app ", log.LstdFlags)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	store, err := scheduler.OpenStore(cfg.DatabasePath)
	if err != nil {
		logger.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()

	registryStore := registry.NewStore(cfg.RegistryPath)

	s3Client, err := s3backup.New(context.Background(), cfg.S3)
	if err != nil {
		logger.Fatalf("create s3 client: %v", err)
	}

	// engineFactory builds a fresh shelfcore.Engine per backup request, wiring
	// the reporter provided by backupservice so each run captures its own log.
	engineFactory := func(reporter shelfcore.Reporter) *shelfcore.Engine {
		return shelfcore.New(registryStore, cfg.BackupsDir, reporter)
	}
	backups := backupservice.New(engineFactory, cfg.BackupsDir, s3Client, logger)
	if backups.S3Enabled() {
		logger.Printf("s3 backup uploads enabled: %s", backups.S3Destination())
	}

	manager, err := scheduler.NewManager(store, backups, cfg.Timezone, logger)
	if err != nil {
		logger.Fatalf("create scheduler: %v", err)
	}
	if err := manager.Reload(context.Background()); err != nil {
		logger.Fatalf("load schedules: %v", err)
	}
	manager.Start()
	defer manager.Stop(context.Background())

	handler, err := server.New(cfg, backups, store, manager, logger)
	if err != nil {
		logger.Fatalf("create server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("listening on http://%s", cfg.Addr)
		if cfg.UsingDefaultPassword {
			logger.Printf("using default basic auth credentials admin/admin; set APP_USERNAME and APP_PASSWORD")
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Printf("shutdown: %v", err)
	}
}
