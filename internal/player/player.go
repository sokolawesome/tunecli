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

func (player *Player) LoadFile(path string) error {
	command := map[string]any{"command": []string{"loadfile", path, "replace"}}
	jsonCommand, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal mpv command: %s", err)
	}

	_, err = player.Conn.Write(append(jsonCommand, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write to connection: %s", err)
	}

	return nil
}

func (player *Player) Close() {
	if err := player.Conn.Close(); err != nil {
		log.Printf("error closing connection: %s", err)
	}
	if err := player.cmd.Process.Kill(); err != nil {
		log.Printf("error killing mpv process: %s", err)
	}
}
