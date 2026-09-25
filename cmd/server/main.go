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
	"github.com/jeffwnelson/sodatracker/internal/config"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/logging"
	"github.com/jeffwnelson/sodatracker/internal/notify"
	"github.com/jeffwnelson/sodatracker/internal/seed"
	"github.com/jeffwnelson/sodatracker/internal/service"
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
	// Wire: stock alerts → Slack. No-op unless SLACK_WEBHOOK_URL is set.
	notify.RegisterSlack(bus, st, notify.NewSlack(cfg.SlackWebhookURL))
	if cfg.SlackWebhookURL != "" {
		slog.Info("Slack notifications enabled")
	}

	router := api.NewRouter(st, svc, api.RouterConfig{
		APIKey:             cfg.APIKey,
		JWTSecret:          cfg.JWTSecret,
		StaticDir:          cfg.StaticDir,
		SlackSigningSecret: cfg.SlackSigningSecret,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("SodaTracker API starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
