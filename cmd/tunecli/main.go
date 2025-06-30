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
	log.Println("Loading configurations...")
	cfg, err := config.LoadConfigs()
	if err != nil {
		log.Fatalf("error loading config: %v\n", err)
	}

	log.Println("Starting media player...")
	p, err := player.NewPlayer()
	if err != nil {
		log.Fatalf("error starting player: %v\n", err)
	}
	defer func() {
		log.Println("Shutting down player...")
		if err := p.Shutdown(); err != nil {
			log.Printf("error during player shutdown: %v", err)
		}
	}()

	log.Println("Starting MPRIS integration...")
	mprisServer, err := mpris.NewServer(p)
	if err != nil {
		log.Printf("failed to start MPRIS server: %v\n", err)
	}
	if mprisServer != nil {
		log.Println("MPRIS server started successfully.")
		defer mprisServer.Shutdown()
	}

	model := ui.NewModel(p, cfg)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Printf("error: %v", err)
		os.Exit(1)
	}

	log.Println("Application exiting.")
}
