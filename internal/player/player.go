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
	Duration  float64
}

type mpvEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Data  any    `json:"data"`
}

type Player struct {
	cmd          *exec.Cmd
	StateChanges chan State
	mu           sync.RWMutex
	state        State
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func New() (*Player, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Player{
		StateChanges: make(chan State, 10),
		ctx:          ctx,
		cancel:       cancel,
		state:        State{Volume: 100},
	}

	if err := p.start(); err != nil {
		cancel()
		return nil, err
	}

	return p, nil
}

func (p *Player) start() error {
	if err := p.cleanupSocket(); err != nil {
		return fmt.Errorf("cleanup socket: %w", err)
	}

	p.cmd = exec.CommandContext(p.ctx, "mpv",
		"--idle",
		"--input-ipc-server="+socketPath,
		"--no-video",
		"--no-terminal",
	)

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start mpv: %w", err)
	}

	if err := p.waitForSocket(); err != nil {
		p.kill()
		return fmt.Errorf("socket timeout: %w", err)
	}

	if err := p.setupObservers(); err != nil {
		p.kill()
		return fmt.Errorf("setup observers: %w", err)
	}

	p.wg.Add(1)
	go p.eventLoop(stdout)

	return nil
}

func (p *Player) cleanupSocket() error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return err
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
			return fmt.Errorf("timeout")
		case <-ticker.C:
			if conn, err := net.Dial("unix", socketPath); err == nil {
				conn.Close()
				return nil
			}
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

func (p *Player) setupObservers() error {
	properties := []string{"pause", "media-title", "volume", "time-pos", "duration"}
	for i, prop := range properties {
		if err := p.sendCommand([]any{"observe_property", i + 1, prop}); err != nil {
			return fmt.Errorf("observe %s: %w", prop, err)
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
			p.processEvent(scanner.Text())
		}
	}
}

func (p *Player) processEvent(line string) {
	var event mpvEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return
	}

	p.mu.Lock()
	var updated bool

	switch event.Event {
	case "property-change":
		updated = p.updateState(event.Name, event.Data)
	case "playback-restart":
		p.state.IsPlaying = true
		updated = true
	case "idle":
		p.state.IsPlaying = false
		updated = true
	case "end-file":
		p.state.IsPlaying = false
		updated = true
	}

	if updated {
		state := p.state
		p.mu.Unlock()
		select {
		case p.StateChanges <- state:
		case <-p.ctx.Done():
		default:
		}
	} else {
		p.mu.Unlock()
	}
}

func (p *Player) updateState(name string, data any) bool {
	switch name {
	case "pause":
		if paused, ok := data.(bool); ok {
			p.state.IsPlaying = !paused
			return true
		}
	case "media-title":
		if title, ok := data.(string); ok {
			p.state.Title = strings.TrimSpace(title)
			return true
		}
	case "volume":
		if volume, ok := data.(float64); ok {
			p.state.Volume = int(volume)
			return true
		}
	case "duration":
		if dur, ok := data.(float64); ok {
			p.state.Duration = dur
			return true
		}
	}
	return false
}

func (p *Player) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *Player) LoadFile(path, mode string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty path")
	}

	if err := p.sendCommand([]any{"loadfile", path, mode}); err != nil {
		return err
	}

	p.mu.Lock()
	p.state.IsPlaying = true
	state := p.state
	p.mu.Unlock()

	select {
	case p.StateChanges <- state:
	case <-p.ctx.Done():
	default:
	}

	return nil
}

func (p *Player) TogglePause() error {
	if err := p.sendCommand([]any{"cycle", "pause"}); err != nil {
		return err
	}

	p.mu.Lock()
	p.state.IsPlaying = !p.state.IsPlaying
	state := p.state
	p.mu.Unlock()

	select {
	case p.StateChanges <- state:
	case <-p.ctx.Done():
	default:
	}

	return nil
}

func (p *Player) Stop() error {
	if err := p.sendCommand([]any{"stop"}); err != nil {
		return err
	}

	p.mu.Lock()
	p.state.IsPlaying = false
	p.state.Title = ""
	p.state.Duration = 0
	state := p.state
	p.mu.Unlock()

	select {
	case p.StateChanges <- state:
	case <-p.ctx.Done():
	default:
	}

	return nil
}

func (p *Player) SetVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("volume must be 0-100")
	}
	return p.sendCommand([]any{"set_property", "volume", volume})
}

func (p *Player) Seek(seconds float64) error {
	return p.sendCommand([]any{"seek", seconds})
}

func (p *Player) sendCommand(command []any) error {
	ctx, cancel := context.WithTimeout(p.ctx, commandTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	cmdBytes, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if _, err := conn.Write(append(cmdBytes, '\n')); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (p *Player) kill() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func (p *Player) Shutdown() error {
	p.cancel()

	done := make(chan error, 1)
	go func() {
		done <- p.sendCommand([]any{"quit"})
	}()

	select {
	case err := <-done:
		if err != nil {
			p.kill()
		}
	case <-time.After(2 * time.Second):
		p.kill()
	}

	p.wg.Wait()

	if p.cmd != nil {
		p.cmd.Wait()
	}

	close(p.StateChanges)
	p.cleanupSocket()

	return nil
}
