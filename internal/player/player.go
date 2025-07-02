package player

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	socketPath     = "/tmp/tunecli-mpv.sock"
	startupTimeout = 5 * time.Second
	commandTimeout = 3 * time.Second
)

type State struct {
	IsPlaying bool
	Title     string
	Volume    int
	Position  float64
	Duration  float64
}

type MpvEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Data  any    `json:"data"`
}

type Player struct {
	cmd          *exec.Cmd
	StateChanges chan State
	mu           sync.RWMutex
	currentState State
	ctx          context.Context
	cancel       context.CancelFunc
	started      bool
	wg           sync.WaitGroup
}

func NewPlayer() (*Player, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Player{
		StateChanges: make(chan State, 10),
		ctx:          ctx,
		cancel:       cancel,
		currentState: State{Volume: 100},
	}

	if err := p.start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start player: %w", err)
	}

	return p, nil
}

func (p *Player) start() error {
	if err := p.cleanupSocket(); err != nil {
		return fmt.Errorf("failed to cleanup socket: %w", err)
	}

	cmd := exec.CommandContext(p.ctx, "mpv",
		"--idle",
		"--input-ipc-server="+socketPath,
		"--no-video",
		"--no-terminal",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not get stdout pipe from mpv: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start mpv: %w", err)
	}

	p.cmd = cmd
	p.started = true

	if err := p.waitForSocket(); err != nil {
		p.killProcess()
		return fmt.Errorf("mpv socket not ready: %w", err)
	}

	if err := p.setupPropertyObservation(); err != nil {
		p.killProcess()
		return fmt.Errorf("could not setup property observation: %w", err)
	}

	p.wg.Add(1)
	go p.eventLoop(stdout)

	return nil
}

func (p *Player) cleanupSocket() error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove old socket file: %w", err)
	}
	return nil
}

func (p *Player) waitForSocket() error {
	timeout := time.After(startupTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for mpv socket")
		case <-ticker.C:
			if _, err := net.Dial("unix", socketPath); err == nil {
				return nil
			}
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

func (p *Player) setupPropertyObservation() error {
	properties := []string{"pause", "media-title", "volume", "time-pos", "duration"}

	for i, prop := range properties {
		if err := p.sendCommand([]any{"observe_property", i + 1, prop}); err != nil {
			return fmt.Errorf("failed to observe property %s: %w", prop, err)
		}
	}

	return nil
}

func (p *Player) eventLoop(stdout io.ReadCloser) {
	defer p.wg.Done()
	defer stdout.Close()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-p.ctx.Done():
			return
		default:
			line := scanner.Text()
			if err := p.processEvent(line); err != nil {
				continue
			}
		}
	}
}

func (p *Player) processEvent(line string) error {
	var event MpvEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return err
	}

	if event.Event != "property-change" {
		return nil
	}

	p.mu.Lock()
	updated := false

	switch event.Name {
	case "pause":
		if paused, ok := event.Data.(bool); ok {
			p.currentState.IsPlaying = !paused
			updated = true
		}
	case "media-title":
		if title, ok := event.Data.(string); ok {
			p.currentState.Title = filepath.Base(title)
			updated = true
		}
	case "volume":
		if volume, ok := event.Data.(float64); ok {
			p.currentState.Volume = int(volume)
			updated = true
		}
	case "time-pos":
		if pos, ok := event.Data.(float64); ok {
			p.currentState.Position = pos
			updated = true
		}
	case "duration":
		if dur, ok := event.Data.(float64); ok {
			p.currentState.Duration = dur
			updated = true
		}
	}

	if updated {
		state := p.currentState
		p.mu.Unlock()

		select {
		case p.StateChanges <- state:
		case <-p.ctx.Done():
		default:
		}
	} else {
		p.mu.Unlock()
	}

	return nil
}

func (p *Player) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentState
}

func isYoutubeURL(path string) bool {
	return strings.Contains(path, "youtube.com") || strings.Contains(path, "youtu.be")
}

func getStreamURLFromYoutube(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", "-f", "ba", "-g", url)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("yt-dlp failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	streamURL := strings.TrimSpace(string(output))
	if streamURL == "" {
		return "", fmt.Errorf("yt-dlp returned empty stream URL")
	}

	return streamURL, nil
}

func (p *Player) LoadFile(path string, mode string) error {
	if !p.started {
		return fmt.Errorf("player not started")
	}

	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if isYoutubeURL(path) {
		streamURL, err := getStreamURLFromYoutube(path)
		if err != nil {
			return fmt.Errorf("could not process youtube url: %w", err)
		}
		path = streamURL
	}

	return p.sendCommand([]any{"loadfile", path, mode})
}

func (p *Player) sendCommand(command []any) error {
	if !p.started {
		return fmt.Errorf("player not started")
	}

	ctx, cancel := context.WithTimeout(p.ctx, commandTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	if err != nil {
		return fmt.Errorf("could not connect to mpv socket: %w", err)
	}
	defer conn.Close()

	cmdBytes, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return fmt.Errorf("could not marshal command: %w", err)
	}

	if _, err := conn.Write(append(cmdBytes, '\n')); err != nil {
		return fmt.Errorf("could not write command: %w", err)
	}

	return nil
}

func (p *Player) TogglePause() error {
	return p.sendCommand([]any{"cycle", "pause"})
}

func (p *Player) Stop() error {
	return p.sendCommand([]any{"stop"})
}

func (p *Player) SetVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100")
	}
	return p.sendCommand([]any{"set_property", "volume", volume})
}

func (p *Player) Seek(seconds float64) error {
	return p.sendCommand([]any{"seek", seconds})
}

func (p *Player) killProcess() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func (p *Player) Shutdown() error {
	if !p.started {
		return nil
	}

	p.cancel()

	gracefulShutdown := make(chan error, 1)
	go func() {
		gracefulShutdown <- p.sendCommand([]any{"quit"})
	}()

	select {
	case err := <-gracefulShutdown:
		if err != nil {
			p.killProcess()
		}
	case <-time.After(2 * time.Second):
		p.killProcess()
	}

	p.wg.Wait()

	if p.cmd != nil {
		p.cmd.Wait()
	}

	close(p.StateChanges)
	p.cleanupSocket()

	return nil
}
