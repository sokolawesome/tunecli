package player

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type State struct {
	IsPlaying bool
	Title     string
	Artist    string
	Album     string
	Volume    int
	Position  float64
	Duration  float64
	Error     string
}

type Player struct {
	cmd         *exec.Cmd
	socketPath  string
	mu          sync.RWMutex
	state       State
	subscribers []chan State
	ctx         context.Context
	cancel      context.CancelFunc
}

type mpvResponse struct {
	Data  any    `json:"data"`
	Error string `json:"error"`
}

type mpvEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Data  any    `json:"data"`
}

func New(socketPath string) (*Player, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Player{
		socketPath:  socketPath,
		ctx:         ctx,
		cancel:      cancel,
		state:       State{Volume: 70},
		subscribers: make([]chan State, 0),
	}

	if err := p.start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start player: %w", err)
	}

	return p, nil
}

func (p *Player) start() error {
	if err := p.cleanupSocket(); err != nil {
		return fmt.Errorf("cleanup socket: %w", err)
	}

	p.cmd = exec.CommandContext(p.ctx, "mpv",
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--input-ipc-server="+p.socketPath,
		"--volume="+fmt.Sprintf("%d", p.state.Volume),
	)

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start mpv: %w", err)
	}

	if err := p.waitForSocket(); err != nil {
		p.cleanup()
		return fmt.Errorf("wait for socket: %w", err)
	}

	go p.observeProperties()

	return nil
}

func (p *Player) cleanupSocket() error {
	if err := os.Remove(p.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (p *Player) waitForSocket() error {
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("socket timeout")
		case <-ticker.C:
			if conn, err := net.Dial("unix", p.socketPath); err == nil {
				conn.Close()
				return nil
			}
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

func (p *Player) observeProperties() {
	properties := map[string]int{
		"pause":       1,
		"media-title": 2,
		"volume":      3,
		"time-pos":    4,
		"duration":    5,
	}

	for prop, id := range properties {
		if err := p.sendCommand([]interface{}{"observe_property", id, prop}); err != nil {
			continue
		}
	}

	go p.eventLoop()
}

func (p *Player) eventLoop() {
	conn, err := net.Dial("unix", p.socketPath)
	if err != nil {
		return
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-p.ctx.Done():
			return
		default:
			p.handleEvent(scanner.Text())
		}
	}
}

func (p *Player) handleEvent(line string) {
	var event mpvEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	switch event.Event {
	case "property-change":
		p.updateProperty(event.Name, event.Data)
	case "playback-restart":
		p.state.IsPlaying = true
		p.state.Error = ""
	case "end-file":
		p.state.IsPlaying = false
	}

	p.notifySubscribers()
}

func (p *Player) updateProperty(name string, value interface{}) {
	switch name {
	case "pause":
		if paused, ok := value.(bool); ok {
			p.state.IsPlaying = !paused
		}
	case "media-title":
		if title, ok := value.(string); ok {
			p.state.Title = title
		}
	case "volume":
		if vol, ok := value.(float64); ok {
			p.state.Volume = int(vol)
		}
	case "time-pos":
		if pos, ok := value.(float64); ok {
			p.state.Position = pos
		}
	case "duration":
		if dur, ok := value.(float64); ok {
			p.state.Duration = dur
		}
	}
}

func (p *Player) notifySubscribers() {
	state := p.state
	for _, ch := range p.subscribers {
		select {
		case ch <- state:
		default:
		}
	}
}

func (p *Player) Subscribe() <-chan State {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch := make(chan State, 10)
	p.subscribers = append(p.subscribers, ch)
	return ch
}

func (p *Player) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *Player) Play(path string) error {
	return p.sendCommand([]any{"loadfile", path, "replace"})
}

func (p *Player) TogglePause() error {
	return p.sendCommand([]any{"cycle", "pause"})
}

func (p *Player) Stop() error {
	return p.sendCommand([]any{"stop"})
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
	conn, err := net.Dial("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer conn.Close()

	cmd := map[string]any{"command": command}
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	if _, err := conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write command: %w", err)
	}

	return nil
}

func (p *Player) cleanup() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func (p *Player) Shutdown() {
	p.cancel()

	p.sendCommand([]any{"quit"})

	if p.cmd != nil {
		p.cmd.Wait()
	}

	p.cleanupSocket()
}
