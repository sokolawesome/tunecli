package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/mpris"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/ui"
)

func main() {
	cfg, err := config.LoadConfig()
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

	model, err := ui.NewModel(player, cfg.MusicDirs, cmdChan, server)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	program := tea.NewProgram(model)

	if _, err := program.Run(); err != nil {
		log.Fatalf("error: %s", err)
	}
}
