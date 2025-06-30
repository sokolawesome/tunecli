package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
)

type playerStateMsg player.State

type LogMsg string

type model struct {
	player      *player.Player
	stations    []config.RadioStation
	playerState player.State
	logView     viewport.Model
	logMessages []string
	cursor      int
	keys        KeyMap
	ready       bool
	width       int
	height      int
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

func waitForPlayerChanges(ch chan player.State) tea.Cmd {
	return func() tea.Msg {
		return playerStateMsg(<-ch)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		waitForPlayerChanges(m.player.StateChanges),
		tea.EnterAltScreen,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		topHeight := len(m.stations) + 6
		logHeight := msg.Height - topHeight
		if logHeight < 5 {
			logHeight = 5
		}

		m.logView = viewport.New(msg.Width, logHeight)
		m.logView.SetContent(strings.Join(m.logMessages, "\n"))
		m.logView.GotoBottom()
		m.ready = true

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
			if len(m.stations) > 0 {
				station := m.stations[m.cursor]
				if err := m.player.LoadFile(station.URL, "replace"); err != nil {
					m.logMessages = append(m.logMessages, fmt.Sprintf("Error playing station: %v", err))
					if m.ready {
						m.logView.SetContent(strings.Join(m.logMessages, "\n"))
						m.logView.GotoBottom()
					}
				}
			}
		}

	case playerStateMsg:
		m.playerState = player.State(msg)
		cmds = append(cmds, waitForPlayerChanges(m.player.StateChanges))

	case LogMsg:
		m.logMessages = append(m.logMessages, string(msg))
		if m.ready {
			m.logView.SetContent(strings.Join(m.logMessages, "\n"))
			m.logView.GotoBottom()
		}
	}

	if m.ready {
		m.logView, cmd = m.logView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing TUI..."
	}

	if len(m.stations) == 0 {
		return "No radio stations configured. Please check your stations.yml file."
	}

	var stationList strings.Builder
	stationList.WriteString("TuneCLI - Radio Stations\n\n")

	for i, station := range m.stations {
		cursor := "  "
		if m.cursor == i {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ")
		}

		line := station.Name
		if m.playerState.Title != "" && strings.Contains(strings.ToLower(station.Name), strings.ToLower(m.playerState.Title)) {
			status := " [Paused]"
			if m.playerState.IsPlaying {
				status = " [Playing]"
			}
			line += lipgloss.NewStyle().Foreground(lipgloss.Color("70")).Render(status)
		}
		stationList.WriteString(cursor + line + "\n")
	}

	stationList.WriteString("\nControls: ↑/↓ navigate, Enter play, q quit\n")

	logHeader := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		Width(m.width).
		Render("Logs")

	return stationList.String() + "\n" + logHeader + "\n" + m.logView.View()
}
