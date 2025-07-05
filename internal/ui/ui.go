package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
	"github.com/sokolawesome/tunecli/internal/scanner"
)

type UI struct {
	app         *tview.Application
	config      *config.Config
	player      *player.Player
	musicFiles  []scanner.MusicFile
	currentView ViewType

	layout      *tview.Flex
	stationList *tview.List
	fileList    *tview.List
	statusBar   *tview.TextView
	controlBar  *tview.TextView

	stateChannel <-chan player.State
}

type ViewType int

const (
	StationView ViewType = iota
	FileView
)

func New(cfg *config.Config, p *player.Player) *UI {
	ui := &UI{
		app:          tview.NewApplication(),
		config:       cfg,
		player:       p,
		currentView:  StationView,
		stateChannel: p.Subscribe(),
	}

	ui.setupUI()
	ui.setupKeybinds()
	ui.scanMusicFiles()

	go ui.handleStateUpdates()

	return ui
}

func (ui *UI) setupUI() {
	ui.stationList = tview.NewList().
		SetHighlightFullLine(true).
		SetSelectedTextColor(tcell.ColorBlack).
		SetSelectedBackgroundColor(tcell.ColorWhite)

	ui.fileList = tview.NewList().
		SetHighlightFullLine(true).
		SetSelectedTextColor(tcell.ColorBlack).
		SetSelectedBackgroundColor(tcell.ColorWhite)

	ui.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true)

	ui.controlBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	ui.populateStations()
	ui.updateStatusBar()
	ui.updateControlBar()

	ui.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.getCurrentList(), 0, 1, true).
		AddItem(ui.statusBar, 3, 0, false).
		AddItem(ui.controlBar, 1, 0, false)
}

func (ui *UI) getCurrentList() *tview.List {
	if ui.currentView == StationView {
		return ui.stationList
	}
	return ui.fileList
}

func (ui *UI) populateStations() {
	ui.stationList.Clear()
	for i, station := range ui.config.Stations {
		tags := ""
		if len(station.Tags) > 0 {
			tags = fmt.Sprintf(" [gray](%s)", strings.Join(station.Tags, ", "))
		}

		text := fmt.Sprintf("%s%s", station.Name, tags)
		ui.stationList.AddItem(text, station.URL, 0, func() {
			ui.playStation(i)
		})
	}
}

func (ui *UI) populateFiles() {
	ui.fileList.Clear()
	for _, file := range ui.musicFiles {
		name := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
		dir := filepath.Base(file.Dir)
		text := fmt.Sprintf("%s [gray](%s)", name, dir)

		ui.fileList.AddItem(text, file.Path, 0, func() {
			currentIndex := ui.fileList.GetCurrentItem()
			if currentIndex >= 0 && currentIndex < len(ui.musicFiles) {
				ui.playFile(ui.musicFiles[currentIndex].Path)
			}
		})
	}
}

func (ui *UI) scanMusicFiles() {
	go func() {
		files, err := scanner.ScanDirectories(ui.config.MusicDirs)
		if err != nil {
			return
		}

		ui.musicFiles = files
		ui.app.QueueUpdateDraw(func() {
			ui.populateFiles()
		})
	}()
}

func (ui *UI) setupKeybinds() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			ui.app.Stop()
			return nil
		case ' ':
			ui.player.TogglePause()
			return nil
		case 's':
			ui.player.Stop()
			return nil
		case '+', '=':
			state := ui.player.GetState()
			if state.Volume < 100 {
				ui.player.SetVolume(state.Volume + 5)
			}
			return nil
		case '-':
			state := ui.player.GetState()
			if state.Volume > 0 {
				ui.player.SetVolume(state.Volume - 5)
			}
			return nil
		}

		switch event.Key() {
		case tcell.KeyTab:
			ui.switchView()
			return nil
		case tcell.KeyEnter:
			ui.playSelected()
			return nil
		}

		return event
	})
}

func (ui *UI) switchView() {
	if ui.currentView == StationView {
		ui.currentView = FileView
		ui.layout.RemoveItem(ui.stationList)
		ui.layout.AddItem(ui.fileList, 0, 1, true)
		ui.app.SetFocus(ui.fileList)
	} else {
		ui.currentView = StationView
		ui.layout.RemoveItem(ui.fileList)
		ui.layout.AddItem(ui.stationList, 0, 1, true)
		ui.app.SetFocus(ui.stationList)
	}
}

func (ui *UI) playSelected() {
	if ui.currentView == StationView {
		index := ui.stationList.GetCurrentItem()
		if index >= 0 && index < len(ui.config.Stations) {
			ui.playStation(index)
		}
	} else {
		index := ui.fileList.GetCurrentItem()
		if index >= 0 && index < len(ui.musicFiles) {
			ui.playFile(ui.musicFiles[index].Path)
		}
	}
}

func (ui *UI) playStation(index int) {
	if index >= 0 && index < len(ui.config.Stations) {
		station := ui.config.Stations[index]
		ui.player.Play(station.URL)
	}
}

func (ui *UI) playFile(path string) {
	ui.player.Play(path)
}

func (ui *UI) handleStateUpdates() {
	for state := range ui.stateChannel {
		ui.app.QueueUpdateDraw(func() {
			ui.updateStatusBar()
			ui.updateControlBar()
		})
		_ = state
	}
}

func (ui *UI) updateStatusBar() {
	state := ui.player.GetState()

	status := "Stopped"
	if state.IsPlaying {
		status = "[green]Playing"
	} else if state.Title != "" {
		status = "[yellow]Paused"
	}

	title := state.Title
	if title == "" {
		title = "No media"
	}

	position := formatTime(state.Position)
	duration := formatTime(state.Duration)

	statusText := fmt.Sprintf("%s: %s\nVolume: %d%% | %s / %s",
		status, title, state.Volume, position, duration)

	ui.statusBar.SetText(statusText)
}

func (ui *UI) updateControlBar() {
	viewName := "Stations"
	if ui.currentView == FileView {
		viewName = "Files"
	}

	controls := fmt.Sprintf("Tab: Switch (%s) | Enter: Play | Space: Pause | S: Stop | +/-: Volume | Q: Quit",
		viewName)

	ui.controlBar.SetText(controls)
}

func formatTime(seconds float64) string {
	if seconds <= 0 {
		return "00:00"
	}

	duration := time.Duration(seconds) * time.Second
	minutes := int(duration.Minutes())
	secs := int(duration.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func (ui *UI) Run() error {
	return ui.app.SetRoot(ui.layout, true).Run()
}

func (ui *UI) Stop() {
	ui.app.Stop()
}
