package ui

import (
	"fmt"
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

type stateMsg player.State

type viewMode int

const (
	radioView viewMode = iota
	localFilesView
)

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true,
	".wav": true, ".m4a": true,
}

type Model struct {
	player      *player.Player
	stations    []config.RadioStation
	localFiles  []string
	musicDirs   []string
	playerState player.State
	logView     viewport.Model
	logs        []string
	mode        viewMode
	cursor      int
	width       int
	height      int
	ready       bool
	keys        keyMap
}

type keyMap struct {
	Up, Down, Quit, Play, Pause, SwitchView key.Binding
}

func (k keyMap) ShortHelp() []key.Help {
	return []key.Help{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "enter", Desc: "play"},
		{Key: "space", Desc: "pause"},
		{Key: "tab", Desc: "switch"},
		{Key: "q", Desc: "quit"},
	}
}

func (k keyMap) FullHelp() [][]key.Help {
	return [][]key.Help{k.ShortHelp()}
}

func defaultKeys() keyMap {
	return keyMap{
		Up:         key.NewBinding(key.WithKeys("k", "up")),
		Down:       key.NewBinding(key.WithKeys("j", "down")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c")),
		Play:       key.NewBinding(key.WithKeys("enter")),
		Pause:      key.NewBinding(key.WithKeys("space")),
		SwitchView: key.NewBinding(key.WithKeys("tab")),
	}
}

func New(p *player.Player, cfg *config.Config) Model {
	m := Model{
		player:    p,
		stations:  cfg.Stations,
		musicDirs: cfg.MusicDirs,
		keys:      defaultKeys(),
	}
	m.scanLocalFiles()
	return m
}

func (m *Model) scanLocalFiles() {
	m.localFiles = []string{}
	for _, dir := range m.musicDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if audioExts[strings.ToLower(filepath.Ext(path))] {
				m.localFiles = append(m.localFiles, path)
			}
			return nil
		})
	}
}

func (m Model) Init() tea.Cmd {
	return waitForState(m.player.StateChanges)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.logView = viewport.New(m.width, m.height/3)
		m.logView.SetContent(strings.Join(m.logs, "\n"))
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
			limit := m.getItemCount() - 1
			if m.cursor < limit {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Play):
			m.playSelected()
		case key.Matches(msg, m.keys.Pause):
			m.player.TogglePause()
		}

	case stateMsg:
		m.playerState = player.State(msg)
		cmds = append(cmds, waitForState(m.player.StateChanges))
	}

	if m.ready {
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) getItemCount() int {
	if m.mode == radioView {
		return len(m.stations)
	}
	return len(m.localFiles)
}

func (m *Model) playSelected() {
	var path string
	switch m.mode {
	case radioView:
		if m.cursor < len(m.stations) {
			path = m.stations[m.cursor].URL
		}
	case localFilesView:
		if m.cursor < len(m.localFiles) {
			path = m.localFiles[m.cursor]
		}
	}
	if path != "" {
		m.player.LoadFile(path, "replace")
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	header := m.renderHeader()
	content := m.renderContent()
	status := m.renderStatus()
	help := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left,
		header, content, status, help, m.logView.View())
}

func (m Model) renderHeader() string {
	title := "TuneCLI"
	if m.mode == radioView {
		title += " - Radio Stations"
	} else {
		title += " - Local Files"
	}
	return lipgloss.NewStyle().Bold(true).Render(title) + "\n"
}

func (m Model) renderContent() string {
	var items []string

	switch m.mode {
	case radioView:
		for i, station := range m.stations {
			items = append(items, m.renderItem(station.Name, i))
		}
	case localFilesView:
		start, end := m.getVisibleRange()
		for i := start; i < end; i++ {
			name := filepath.Base(m.localFiles[i])
			items = append(items, m.renderItem(name, i))
		}
	}

	if len(items) == 0 {
		return "No items found.\n"
	}

	return strings.Join(items, "")
}

func (m Model) getVisibleRange() (int, int) {
	const visibleItems = 20
	start := m.cursor - visibleItems/2
	if start < 0 {
		start = 0
	}
	end := start + visibleItems
	if end > len(m.localFiles) {
		end = len(m.localFiles)
		start = end - visibleItems
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func (m Model) renderItem(name string, index int) string {
	cursor := "  "
	if index == m.cursor {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ")
	}

	style := lipgloss.NewStyle()

	isCurrentlyPlaying := false
	if m.playerState.IsPlaying && m.playerState.Title != "" {
		switch m.mode {
		case radioView:
			if index < len(m.stations) {
				isCurrentlyPlaying = strings.Contains(m.playerState.Title, m.stations[index].Name) ||
					strings.Contains(m.stations[index].URL, m.playerState.Title)
			}
		case localFilesView:
			if index < len(m.localFiles) {
				isCurrentlyPlaying = strings.Contains(m.playerState.Title, filepath.Base(m.localFiles[index])) ||
					strings.Contains(m.localFiles[index], m.playerState.Title)
			}
		}
	}

	if isCurrentlyPlaying {
		style = style.Foreground(lipgloss.Color("70"))
		name += " [♪ Playing]"
	}

	return cursor + style.Render(name) + "\n"
}

func (m Model) renderStatus() string {
	if m.playerState.Title == "" {
		return "\nStatus: Stopped\n"
	}

	status := "Paused"
	statusColor := lipgloss.Color("208")
	if m.playerState.IsPlaying {
		status = "Playing"
		statusColor = lipgloss.Color("70")
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	return fmt.Sprintf("\n%s: %s | Volume: %d%%\n",
		statusStyle.Render(status),
		titleStyle.Render(m.playerState.Title),
		m.playerState.Volume)
}

func (m Model) renderHelp() string {
	help := "Controls: ↑/↓ navigate | Enter play | Space pause/resume | Tab switch view | q quit"
	return lipgloss.NewStyle().Faint(true).Render(help) + "\n"
}

func waitForState(ch chan player.State) tea.Cmd {
	return func() tea.Msg {
		return stateMsg(<-ch)
	}
}
