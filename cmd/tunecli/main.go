package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/ui"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %s", err)
	}

	player, err := player.NewPlayer()
	if err != nil {
		log.Fatalf("failed to create player: %s", err)
	}
	defer player.Close()

	model, err := ui.NewModel(player, cfg.MusicDirs)
	if err != nil {
		log.Fatalf("failed to create model: %s", err)
	}

	program := tea.NewProgram(model)

	if _, err := program.Run(); err != nil {
		log.Fatalf("failed to run tui: %s", err)
	}
}
