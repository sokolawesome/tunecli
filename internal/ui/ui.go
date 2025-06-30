package ui

import (
	"log"
	"os"
	"path/filepath"
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

type viewMode int

const (
	radioView viewMode = iota
	localFilesView
)

var audioExtensions = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".ogg":  {},
	".wav":  {},
	".m4a":  {},
}

type model struct {
	player                             *player.Player
	stations                           []config.RadioStation
	playerState                        player.State
	logView                            viewport.Model
	mode                               viewMode
	logMessages, musicDirs, localFiles []string
	localFilesErr                      error
	keys                               KeyMap
	ready                              bool
	width, height, cursor              int
}

type KeyMap struct {
	Up, Down, Quit, Play, Pause, SwitchView key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "move up")),
		Down:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "move down")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Play:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play selected")),
		Pause:      key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pause/resume")),
		SwitchView: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch view")),
	}
}

func NewModel(p *player.Player, cfg *config.AppConfig) model {
	m := model{
		player:    p,
		stations:  cfg.Stations,
		keys:      DefaultKeyMap(),
		musicDirs: cfg.General.MusicDirs,
	}

	m.scanLocalFiles()
	return m
}

func (m *model) scanLocalFiles() {
	m.localFiles = []string{}
	for _, dir := range m.musicDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if _, ok := audioExtensions[filepath.Ext(path)]; ok {
					m.localFiles = append(m.localFiles, path)
				}
			}
			return nil
		})
		if err != nil {
			m.localFilesErr = err
			log.Printf("Error scanning local files in %s: %v", dir, err)
			return
		}
	}
	log.Printf("Found %d local audio files.", len(m.localFiles))
}

func waitForPlayerChanges(ch chan player.State) tea.Cmd {
	return func() tea.Msg {
		return playerStateMsg(<-ch)
	}
}

func (m model) Init() tea.Cmd {
	return waitForPlayerChanges(m.player.StateChanges)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		listHeight := m.height - 10
		m.logView = viewport.New(m.width, m.height-listHeight-2)
		m.logView.SetContent(strings.Join(m.logMessages, "\n"))
		m.ready = true

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.SwitchView):
			m.mode = (m.mode + 1) % 2
			m.cursor = 0
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			var limit int
			if m.mode == radioView {
				limit = len(m.stations) - 1
			} else {
				limit = len(m.localFiles) - 1
			}
			if m.cursor < limit {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Play):
			var path string
			if m.mode == radioView && len(m.stations) > m.cursor {
				path = m.stations[m.cursor].URL
			} else if m.mode == localFilesView && len(m.localFiles) > m.cursor {
				path = m.localFiles[m.cursor]
			}
			if path != "" {
				if err := m.player.LoadFile(path, "replace"); err != nil {
					log.Printf("Error playing item: %v", err)
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
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var mainContent string
	if m.mode == radioView {
		mainContent = m.renderRadioView()
	} else {
		mainContent = m.renderLocalFilesView()
	}

	help := "Controls: ↑/↓ navigate | Enter play | Tab switch view | q quit"
	logHeader := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).Width(m.width).Render("Logs")

	return lipgloss.JoinVertical(lipgloss.Left, mainContent, help, logHeader, m.logView.View())
}

func (m model) renderRadioView() string {
	var b strings.Builder
	b.WriteString("TuneCLI - Radio Stations\n\n")
	for i, station := range m.stations {
		b.WriteString(renderLine(station.Name, i == m.cursor, m.playerState.IsPlaying && m.playerState.Title == station.Name))
	}
	return b.String()
}

func (m model) renderLocalFilesView() string {
	var b strings.Builder
	b.WriteString("TuneCLI - Local Files\n\n")
	if m.localFilesErr != nil {
		b.WriteString("Error reading files. Check logs.")
	} else if len(m.localFiles) == 0 {
		b.WriteString("No local files found in configured directories.")
	} else {
		start := m.cursor - 10
		if start < 0 {
			start = 0
		}
		end := start + 20
		if end > len(m.localFiles) {
			end = len(m.localFiles)
		}

		for i := start; i < end; i++ {
			b.WriteString(renderLine(filepath.Base(m.localFiles[i]), i == m.cursor, m.playerState.IsPlaying && m.playerState.Title == filepath.Base(m.localFiles[i])))
		}
	}

	return b.String()
}

func renderLine(text string, isCursor bool, isPlaying bool) string {
	cursor := "  "
	if isCursor {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ")
	}
	if isPlaying {
		text += lipgloss.NewStyle().Foreground(lipgloss.Color("70")).Render(" [Playing]")
	}

	return cursor + text + "\n"
}
