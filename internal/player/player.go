package player

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"time"
)

type Player struct {
	Conn net.Conn
	cmd  *exec.Cmd
}

func NewPlayer() (*Player, error) {
	cmd := exec.Command("mpv",
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--input-ipc-server=/tmp/tunecli-mpv.sock",
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %s", err)
	}

	time.Sleep(200 * time.Millisecond)

	conn, err := net.Dial("unix", "/tmp/tunecli-mpv.sock")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mpv: %s", err)
	}

	return &Player{
		Conn: conn,
		cmd:  cmd,
	}, nil
}

func (player *Player) sendCommand(command map[string]any) error {
	json, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal mpv command: %s", err)
	}

	_, err = player.Conn.Write(append(json, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write to connection: %s", err)
	}

	return nil
}

func (player *Player) LoadFile(path string) error {
	command := map[string]any{"command": []string{"loadfile", path, "replace"}}
	log.Print("Command sent: loadfile")

	return player.sendCommand(command)
}

func (player *Player) TogglePause() error {
	command := map[string]any{"command": []string{"cycle", "pause"}}
	log.Print("Command sent: play/pause")

	return player.sendCommand(command)
}

func (player *Player) Close() {
	if err := player.Conn.Close(); err != nil {
		log.Printf("failed to close connection: %s", err)
	}
	if err := player.cmd.Process.Kill(); err != nil {
		log.Printf("failed to kill mpv process: %s", err)
	}
}
