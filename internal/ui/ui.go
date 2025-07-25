package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/mpris"
	"github.com/sokolawesome/tunecli/internal/player"
)

var selectedItemStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("205")).
	Bold(true)

var paneStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("80"))

const MaxLogHistory = 5
const footerHeight = 10

type Model struct {
	width       int
	height      int
	songs       []string
	cursor      int
	player      *player.Player
	musicDirs   []string
	stations    []config.Stations
	cmdChan     <-chan string
	mprisServer *mpris.MprisServer
	isPlaying   CurrentStatus
	currentView CurrentView
	logs        []string
	logChan     <-chan string
}

type CurrentStatus uint8

const (
	Stopped CurrentStatus = iota
	Playing
	Paused
)

type CurrentView uint8

const (
	Files CurrentView = iota
	Radios
)

type MprisCommand string
type LogMessage string

func NewModel(
	player *player.Player,
	config *config.Config,
	cmdChan <-chan string,
	logChan <-chan string,
	mprisServer *mpris.MprisServer,
) (*Model, error) {
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
		logChan:     logChan,
		mprisServer: mprisServer,
		isPlaying:   Stopped,
		currentView: Files,
	}, nil
}

func (model *Model) Init() tea.Cmd {
	return tea.Batch(waitForMprisCommand(model.cmdChan), waitForLogMessage(model.logChan), tea.SetWindowTitle("tunecli"))
}

func waitForMprisCommand(cmdChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		return MprisCommand(<-cmdChan)
	}
}

func waitForLogMessage(logChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		return LogMessage(<-logChan)
	}
}

func (model *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

			if model.isPlaying == Paused {
				model.player.TogglePause()
			}

			model.mprisServer.SetPlaybackStatus("Playing")
			model.isPlaying = Playing

		case " ":
			model.player.TogglePause()
			switch model.isPlaying {
			case Playing:
				model.mprisServer.SetPlaybackStatus("Paused")
				model.isPlaying = Paused
			case Paused:
				model.mprisServer.SetPlaybackStatus("Playing")
				model.isPlaying = Playing
			}
		}
	case LogMessage:
		model.logs = append(model.logs, string(msg))

		if len(model.logs) > MaxLogHistory {
			model.logs = model.logs[1:]
		}

		return model, waitForLogMessage(model.logChan)

	case MprisCommand:
		if msg == "toggle_pause" && model.isPlaying != Stopped {
			if err := model.player.TogglePause(); err != nil {
				log.Printf("Failed to toggle pause: %v", err)
			}

			switch model.isPlaying {
			case Playing:
				model.mprisServer.SetPlaybackStatus("Paused")
				model.isPlaying = Paused
			case Paused:
				model.mprisServer.SetPlaybackStatus("Playing")
				model.isPlaying = Playing
			}
		}

		return model, waitForMprisCommand(model.cmdChan)

	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height

		return model, nil
	}

	return model, nil
}

func (model *Model) View() string {
	if model.width == 0 {
		return "Initializing..."
	}
	mainContentHeight := model.height - footerHeight

	leftPane := paneStyle.
		Height(mainContentHeight).
		Width(model.width/2 - 3).
		Render(model.renderListPane())

	var status string
	switch model.isPlaying {
	case Playing:
		status = "Playing"
	case Paused:
		status = "Paused"
	case Stopped:
		status = "Stopped"
	}

	rightPane := paneStyle.
		Height(mainContentHeight).
		Width(model.width / 2).
		Render(status)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	keybinds := "Quit: <ctrl+c> | Switch View: tab | Play/Pause: space | Select song/station: enter"
	logs := strings.Join(model.logs, "\n")

	footerContent := lipgloss.NewStyle().
		PaddingTop(1).
		Render(lipgloss.JoinVertical(lipgloss.Center, keybinds, "\n", logs))

	return lipgloss.JoinVertical(lipgloss.Center, mainContent, footerContent)
}

func (model *Model) renderListPane() string {
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
	} else {
		for i, station := range model.stations {
			if i == model.cursor {
				builder.WriteString(selectedItemStyle.Render("> " + station.Name))
			} else {
				builder.WriteString("  " + station.Name)
			}
			builder.WriteString("\n")
		}
	}

	return builder.String()
}
