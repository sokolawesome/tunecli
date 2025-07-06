package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/player"
)

type Model struct {
	songs     []string
	cursor    int
	player    *player.Player
	musicDirs []string
}

func NewModel(player *player.Player, musicDirs []string) (*Model, error) {
	if len(musicDirs) == 0 {
		return nil, fmt.Errorf("no music dirs provied")
	}

	var songs []string

	for _, dir := range musicDirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %s", err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			path := filepath.Join(dir, file.Name())
			songs = append(songs, path)
		}
	}

	return &Model{
		songs:     songs,
		player:    player,
		musicDirs: musicDirs,
	}, nil
}

func (model *Model) Init() tea.Cmd {
	return nil
}

func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return model, tea.Quit
		case "up", "k":
			model.cursor--
			if model.cursor < 0 {
				model.cursor = len(model.songs) - 1
			}
		case "down", "j":
			model.cursor++
			if model.cursor >= len(model.songs) {
				model.cursor = 0
			}
		case "enter":
			model.player.LoadFile(model.songs[model.cursor])
		}
	}

	return model, nil
}

func (model *Model) View() string {
	var builder strings.Builder
	for i, file := range model.songs {
		if i == model.cursor {
			builder.WriteString("> ")
		} else {
			builder.WriteString("  ")
		}
		builder.WriteString(filepath.Base(file) + "\n")
	}
	builder.WriteString("Quit: <ctrl+c>")
	return builder.String()
}
