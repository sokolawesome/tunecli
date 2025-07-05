package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/ui"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	p, err := player.New(cfg.Player.SocketPath)
	if err != nil {
		return fmt.Errorf("create player: %w", err)
	}
	defer p.Shutdown()

	if err := p.SetVolume(cfg.Player.Volume); err != nil {
		log.Printf("Warning: failed to set initial volume: %v", err)
	}

	app := ui.New(cfg, p)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		app.Stop()
	}()

	if err := app.Run(); err != nil {
		return fmt.Errorf("run UI: %w", err)
	}

	return nil
}
