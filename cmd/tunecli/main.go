package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/integration/mpris"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	p, err := player.New()
	if err != nil {
		log.Fatalf("player error: %v", err)
	}
	defer p.Shutdown()

	mprisServer, err := mpris.NewServer(p)
	if err != nil {
		log.Printf("MPRIS failed: %v", err)
	}
	if mprisServer != nil {
		defer mprisServer.Shutdown()
	}

	model := ui.New(p, cfg)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
