package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/logview"
	"github.com/sokolawesome/tunecli/internal/mpris"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/ui"
)

func main() {
	logChan := make(chan string, 20)
	logger := logview.NewLogWriter(logChan)
	log.SetOutput(logger)
	log.SetFlags(0)
	log.Print("tunecli starting...")

	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	cmdChan := make(chan string, 1)

	player, err := player.NewPlayer()
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	defer player.Close()

	server, err := mpris.NewMprisServer(cmdChan)
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	defer server.Close()

	model, err := ui.NewModel(player, config, cmdChan, logChan, server)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		log.Fatalf("error: %s", err)
	}
}
