package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeffwnelson/sodatracker/pkg/gpio"
	"github.com/jeffwnelson/sodatracker/pkg/piagent"
)

func main() {
	configPath := flag.String("config", "piagent.yaml", "path to config file")
	flag.Parse()

	cfg, err := piagent.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Use mock GPIO provider by default (real RPi GPIO requires build tag `rpi`)
	provider := gpio.NewMockProvider()

	agent, err := piagent.New(cfg, provider)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-done
		log.Println("shutting down pi agent...")
		cancel()
	}()

	if err := agent.Run(ctx); err != nil {
		log.Fatalf("agent error: %v", err)
	}
}
