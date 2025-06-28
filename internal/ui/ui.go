package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
)

type model struct {
	player   *player.Player
	stations []config.RadioStation
	cursor   int
	keys     KeyMap
}

type KeyMap struct {
	Up   key.Binding
	Down key.Binding
	Quit key.Binding
	Play key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "move up")),
		Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "move down")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Play: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play selected")),
	}
}

func NewModel(p *player.Player, cfg *config.AppConfig) model {
	return model{
		player:   p,
		stations: cfg.Stations,
		keys:     DefaultKeyMap(),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.stations)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Play):
			station := m.stations[m.cursor]
			_ = m.player.LoadFile(station.URL, "replace")
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "Stations:\n\n"

	for i, station := range m.stations {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, station.Name)
	}

	s += "\nPress 'q' to quit.\n"
	return s
}
