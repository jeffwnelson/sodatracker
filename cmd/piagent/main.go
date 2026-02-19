package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeffwnelson/sodatracker/pkg/piagent"
)

func main() {
	configPath := flag.String("config", "piagent.yaml", "path to config file")
	flag.Parse()

	cfg, err := piagent.LoadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	provider, err := newProvider()
	if err != nil {
		slog.Error("failed to init GPIO provider", "error", err)
		os.Exit(1)
	}

	agent, err := piagent.New(cfg, provider)
	if err != nil {
		slog.Error("failed to create agent", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-done
		slog.Info("shutting down pi agent")
		cancel()
	}()

	if err := agent.Run(ctx); err != nil {
		slog.Error("agent error", "error", err)
		os.Exit(1)
	}
}
