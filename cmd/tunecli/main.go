package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/ui"
)

func main() {
	cfg, err := config.LoadConfigs()
	if err != nil {
		os.Exit(1)
	}

	p, err := player.NewPlayer()
	if err != nil {
		os.Exit(1)
	}
	defer func() {
		if err := p.Shutdown(); err != nil {
		}
	}()

	model := ui.NewModel(p, cfg)
	program := tea.NewProgram(model)

	if _, err := program.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
