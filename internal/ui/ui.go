package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/mpris"
	"github.com/sokolawesome/tunecli/internal/player"
)

var (
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
)

type Model struct {
	songs       []string
	cursor      int
	player      *player.Player
	musicDirs   []string
	stations    []config.Stations
	cmdChan     <-chan string
	mprisServer *mpris.MprisServer
	isPlaying   bool
	currentView CurrentView
}

type CurrentView uint8

const (
	Files CurrentView = iota
	Radios
)

type MprisCommand string

func NewModel(player *player.Player, config *config.Config, cmdChan <-chan string, mprisServer *mpris.MprisServer) (*Model, error) {
	if len(config.MusicDirs) == 0 {
		return nil, fmt.Errorf("no music dirs provied")
	}

	var songs []string

	for _, dir := range config.MusicDirs {
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
		songs:       songs,
		player:      player,
		musicDirs:   config.MusicDirs,
		stations:    config.Stations,
		cmdChan:     cmdChan,
		mprisServer: mprisServer,
		isPlaying:   false,
		currentView: Files,
	}, nil
}

func (model *Model) Init() tea.Cmd {
	return waitForMprisCommand(model.cmdChan)
}

func waitForMprisCommand(cmdChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		return MprisCommand(<-cmdChan)
	}
}

func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MprisCommand:
		if msg == "toggle_pause" {
			if err := model.player.TogglePause(); err != nil {
				fmt.Println("Failed to toggle pause:", err)
			}
			if model.isPlaying {
				model.mprisServer.SetPlaybackStatus("Paused")
			} else {
				model.mprisServer.SetPlaybackStatus("Playing")
			}
			model.isPlaying = !model.isPlaying
		}
		return model, waitForMprisCommand(model.cmdChan)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return model, tea.Quit

		case "tab":
			switch model.currentView {
			case Files:
				model.currentView = Radios
			case Radios:
				model.currentView = Files
			}

			model.cursor = 0

		case "up", "k":
			model.cursor--

			if model.cursor < 0 {
				if model.currentView == Radios {
					model.cursor = len(model.stations) - 1
				} else {
					model.cursor = len(model.songs) - 1
				}
			}

		case "down", "j":
			model.cursor++

			var boundary int
			if model.currentView == Radios {
				boundary = len(model.stations)
			} else {
				boundary = len(model.songs)
			}
			if model.cursor >= boundary {
				model.cursor = 0
			}

		case "enter":
			if model.currentView == Radios {
				model.player.LoadFile(model.stations[model.cursor].Url)
			} else {
				model.player.LoadFile(model.songs[model.cursor])
			}

			model.mprisServer.SetPlaybackStatus("Playing")
			model.isPlaying = true

		case " ":
			model.player.TogglePause()
			if model.isPlaying {
				model.mprisServer.SetPlaybackStatus("Paused")
			} else {
				model.mprisServer.SetPlaybackStatus("Playing")
			}

			model.isPlaying = !model.isPlaying
		}
	}

	return model, nil
}

func (model *Model) View() string {
	var builder strings.Builder

	if model.currentView == Files {
		for i, song := range model.songs {
			song = filepath.Base(song)
			if i == model.cursor {
				builder.WriteString(selectedItemStyle.Render("> " + song))
			} else {
				builder.WriteString("  " + song)
			}
			builder.WriteString("\n")
		}

		builder.WriteString("Quit: <ctrl+c>")

		return builder.String()
	} else {
		for i, station := range model.stations {
			if i == model.cursor {
				builder.WriteString(selectedItemStyle.Render("> " + station.Name))
			} else {
				builder.WriteString("  " + station.Name)
			}
			builder.WriteString("\n")
		}

		builder.WriteString("Quit: <ctrl+c>")

		return builder.String()
	}
}
