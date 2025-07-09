package mpris

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	interfaceName = "org.mpris.MediaPlayer2.Player"
	busName       = "org.mpris.MediaPlayer2.tunecli"
	objectPath    = "/org/mpris/MediaPlayer2"
)

type MprisServer struct {
	conn    *dbus.Conn
	CmdChan chan<- string
	props   *prop.Properties
}

func NewMprisServer(cmdChan chan<- string) (*MprisServer, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dbus: %s", err)
	}
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to request bus name: %s", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, fmt.Errorf("failed to become primary owner of bus name")
	}

	server := &MprisServer{conn: conn, CmdChan: cmdChan}

	if err := conn.Export(server, objectPath, interfaceName); err != nil {
		return nil, fmt.Errorf("failed to export player server: %s", err)
	}

	propsSpec := prop.Map{
		interfaceName: {
			"PlaybackStatus": {
				Value:    "Stopped",
				Writable: false,
				Emit:     prop.EmitTrue,
			},
		},
	}

	props, err := prop.Export(conn, objectPath, propsSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to export properties: %s", err)
	}

	server.props = props

	return server, nil
}

func (server *MprisServer) SetPlaybackStatus(status string) error {
	if err := server.props.Set(interfaceName, "PlaybackStatus", dbus.MakeVariant(status)); err != nil {
		return fmt.Errorf("failed to set playback status: %s", err)
	}
	return nil
}

func (server *MprisServer) PlayPause() *dbus.Error {
	server.CmdChan <- "toggle_pause"
	return nil
}

func (server *MprisServer) Close() {
	if err := server.conn.Close(); err != nil {
		log.Printf("failed to close connection: %s", err)
	}
}
