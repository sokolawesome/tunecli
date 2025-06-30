package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const socketPath = "/tmp/tunecli-mpv.sock"

type State struct {
	IsPlaying bool
	Title     string
}

type MpvEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Data  any    `json:"data"`
}

type Player struct {
	cmd          *exec.Cmd
	StateChanges chan State
	currentState State
	shutdown     chan struct{}
}

func NewPlayer() (*Player, error) {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Printf("could not remove old socket file: %v", err)
	}

	cmd := exec.Command("mpv",
		"--idle",
		"--input-ipc-server="+socketPath,
		"--no-video",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stdout pipe from mpv: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start mpv: %w", err)
	}
	time.Sleep(200 * time.Millisecond)

	p := &Player{
		cmd:          cmd,
		StateChanges: make(chan State, 10),
		shutdown:     make(chan struct{}),
	}

	if err := p.observeProperties(); err != nil {
		_ = p.cmd.Process.Kill()
		return nil, fmt.Errorf("could not set up mpv property observation: %w", err)
	}

	go p.eventLoop(stdout)

	return p, nil
}

func (p *Player) observeProperties() error {
	if err := p.sendCommand([]any{"observe_property", 1, "pause"}); err != nil {
		return err
	}
	if err := p.sendCommand([]any{"observe_property", 2, "media-title"}); err != nil {
		return err
	}
	return nil
}

func (p *Player) eventLoop(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-p.shutdown:
			return
		default:
			line := scanner.Text()
			var event MpvEvent
			if err := json.Unmarshal([]byte(line), &event); err == nil && event.Event == "property-change" {
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
				}
				if updated {
					p.StateChanges <- p.currentState
				}
			}
		}
	}
}

func isYoutubeURL(path string) bool {
	return strings.Contains(path, "youtube.com") || strings.Contains(path, "youtu.be")
}

func getStreamURLFromYoutube(url string) (string, error) {
	log.Printf("Processing YouTube URL: %s", url)
	cmd := exec.Command("yt-dlp", "-f", "ba", "-g", url)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("yt-dlp failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run yt-dlp: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (p *Player) LoadFile(path string, mode string) error {
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
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("could not connect to mpv socket: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to close MPV socket connection: %v", err)
		}
	}()

	cmdBytes, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return err
	}

	_, err = conn.Write(append(cmdBytes, '\n'))
	return err
}

func (p *Player) TogglePause() error {
	return p.sendCommand([]any{"cycle", "pause"})
}

func (p *Player) Stop() error {
	return p.sendCommand([]any{"stop"})
}

func (p *Player) Shutdown() error {
	close(p.shutdown)
	if err := p.sendCommand([]any{"quit"}); err != nil {
		log.Printf("graceful quit failed, attempting to kill process: %v", err)
		return p.cmd.Process.Kill()
	}

	return p.cmd.Wait()
}
