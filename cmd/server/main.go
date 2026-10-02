package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Embed the zoneinfo database: the Alpine runtime image has none, and
	// analytics bucket by the caller's timezone.
	_ "time/tzdata"

	"github.com/jeffwnelson/sodatracker/internal/api"
	"github.com/jeffwnelson/sodatracker/internal/backup"
	"github.com/jeffwnelson/sodatracker/internal/buildinfo"
	"github.com/jeffwnelson/sodatracker/internal/config"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/logging"
	"github.com/jeffwnelson/sodatracker/internal/notify"
	"github.com/jeffwnelson/sodatracker/internal/seed"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/slackcmd"
	"github.com/jeffwnelson/sodatracker/internal/slackconf"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func main() {
	cfg := config.Load()
	logging.Setup(cfg.LogFormat)

	st, err := sqlite.New(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	// Seed admin user if not exists
	seed.AdminUser(st, cfg.AdminPassword)

	bus := local.New()
	defer bus.Close()

	svc := service.New(st, cfg.JWTSecret, bus)

	// Wire: dispense event → alert evaluation
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		machineID, _ := evt.Data["machineId"].(string)
		slotPos := 0
		switch v := evt.Data["slotPosition"].(type) {
		case int:
			slotPos = v
		case float64:
			slotPos = int(v)
		}
		if machineID != "" && slotPos > 0 {
			return svc.EvaluateStockAlert(ctx, machineID, slotPos)
		}
		return nil
	})
	// Slack: the webhook (alerts) and app token (/soda over Socket Mode,
	// which dials out, so the Pi needn't be reachable) are edited on the
	// Settings page. SLACK_WEBHOOK_URL and SLACK_APP_TOKEN are copied in
	// once, on the first start that has them.
	slackCtx, stopSlack := context.WithCancel(context.Background())
	defer stopSlack()
	cmds := slackcmd.New(st, svc)
	slack, err := slackconf.New(slackCtx, st, func(ctx context.Context, _, text string) any {
		return cmds.Run(ctx, text)
	}, cfg.SlackWebhookURL, cfg.SlackAppToken)
	if err != nil {
		slog.Error("Slack disabled", "error", err)
	} else {
		notify.RegisterSlackSource(bus, st, slack.Client)
	}

	// Backups. A failure here is logged, not fatal: the machine should keep
	// counting cans even if the backup directory is unusable.
	backupCtx, stopBackups := context.WithCancel(context.Background())
	defer stopBackups()
	// Settings (repo, token, schedule) are edited on the Settings page.
	backups, err := backup.New(backupCtx, backup.Config{Dir: cfg.BackupDir, Bus: bus}, st)
	if err != nil {
		slog.Error("backups disabled", "error", err)
	} else {
		s := backups.Status()
		slog.Info("backups enabled", "dir", cfg.BackupDir, "intervalHours", s.IntervalHours,
			"keep", s.Keep, "repo", s.Repo, "push", s.PushEnabled)
		go backups.Run(backupCtx)
	}

	streamsDone := make(chan struct{})
	router := api.NewRouter(st, svc, api.RouterConfig{
		APIKey:             cfg.APIKey,
		JWTSecret:          cfg.JWTSecret,
		StaticDir:          cfg.StaticDir,
		SlackSigningSecret: cfg.SlackSigningSecret,
		Bus:                bus,
		StreamsDone:        streamsDone,
		Backups:            backups,
		Slack:              slack,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	// Shutdown waits for active requests, and event streams never finish on
	// their own.
	srv.RegisterOnShutdown(func() { close(streamsDone) })

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("SodaTracker API starting", "port", cfg.Port, "version", buildinfo.Version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down")
	stopSlack()
	stopBackups()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
