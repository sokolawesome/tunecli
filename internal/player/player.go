package player

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"
)

const socketPath = "/tmp/tunecli-mpv.sock"

type Player struct {
	cmd *exec.Cmd
}

func NewPlayer() (*Player, error) {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not remove socket file: %v", err)
	}

	cmd := exec.Command("mpv", "--idle", "--input-ipc-server="+socketPath, "--no-video")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start mpv: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	return &Player{cmd: cmd}, nil
}

func (p *Player) sendCommand(command []any) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("could not connect to mpv socket: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Printf("failed to close MPV socket connection: %v", err)
		}
	}()

	cmdBytes, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return err
	}

	_, err = conn.Write(append(cmdBytes, '\n'))
	return err
}

func (p *Player) LoadFile(path string, mode string) error {
	return p.sendCommand([]any{"loadfile", path, mode})
}

func (p *Player) TogglePause() error {
	return p.sendCommand([]any{"cycle", "pause"})
}

func (p *Player) Stop() error {
	return p.sendCommand([]any{"stop"})
}

func (p *Player) Shutdown() error {
	if err := p.sendCommand([]any{"quit"}); err != nil {
		log.Printf("graceful quit failed, attempting to kill process: %v", err)
		return p.cmd.Process.Kill()
	}

	return p.cmd.Wait()
}
