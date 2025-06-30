package mpris

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
	"github.com/sokolawesome/tunecli/internal/player"
)

const interfaceName = "org.mpris.MediaPlayer2.Player"
const busName = "org.mpris.MediaPlayer2.tunecli"

type Server struct {
	conn   *dbus.Conn
	player *player.Player
	props  *prop.Properties
}

func NewServer(p *player.Player) (*Server, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("could not connect to D-Bus session bus: %w", err)
	}

	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, fmt.Errorf("could not request bus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, fmt.Errorf("bus name %s is already taken", busName)
	}

	s := &Server{
		conn:   conn,
		player: p,
	}
	propsSpec := prop.Map{
		interfaceName: {
			"PlaybackStatus": {
				Value:    "Stopped",
				Writable: false,
				Emit:     prop.EmitTrue,
			},
			"Metadata": {
				Value:    map[string]dbus.Variant{},
				Writable: false,
				Emit:     prop.EmitTrue,
			},
			"CanPlay":  {Value: true, Writable: false, Emit: prop.EmitInvalidates},
			"CanPause": {Value: true, Writable: false, Emit: prop.EmitInvalidates},
		},
	}
	s.props, err = prop.Export(conn, "/org/mpris/MediaPlayer2", propsSpec)
	if err != nil {
		return nil, err
	}

	if err := conn.Export(s, "/org/mpris/MediaPlayer2", interfaceName); err != nil {
		return nil, err
	}
	if err := conn.Export(s, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2"); err != nil {
		return nil, err
	}

	go s.watchPlayerState()

	return s, nil
}

func (s *Server) watchPlayerState() {
	for state := range s.player.StateChanges {
		status := "Paused"
		if state.IsPlaying {
			status = "Playing"
		}
		if state.Title == "" {
			status = "Stopped"
		}
		s.props.Set(interfaceName, "PlaybackStatus", dbus.MakeVariant(status))

		metadata := map[string]dbus.Variant{
			"mpris:trackid": dbus.MakeVariant("/org/mpris/MediaPlayer2/track/0"),
			"mpris:length":  dbus.MakeVariant(uint64(0)),
			"xesam:title":   dbus.MakeVariant(state.Title),
			"xesam:artist":  dbus.MakeVariant([]string{}),
			"xesam:album":   dbus.MakeVariant(""),
		}
		s.props.Set(interfaceName, "Metadata", dbus.MakeVariant(metadata))
	}
}

func (s *Server) PlayPause() *dbus.Error {
	log.Println("MPRIS: PlayPause called")
	s.player.TogglePause()
	return nil
}

func (s *Server) Play() *dbus.Error {
	log.Println("MPRIS: Play called")
	val, dberr := s.props.Get(interfaceName, "PlaybackStatus")
	if dberr != nil {
		log.Printf("MPRIS: could not get playback status: %v", dberr)
		return nil
	}
	if status, ok := val.Value().(string); ok && status == "Paused" {
		if err := s.player.TogglePause(); err != nil {
			log.Printf("error toggling pause via MPRIS: %v", err)
		}
	}
	return nil
}

func (s *Server) Pause() *dbus.Error {
	log.Println("MPRIS: Pause called")
	val, dberr := s.props.Get(interfaceName, "PlaybackStatus")
	if dberr != nil {
		log.Printf("MPRIS: could not get playback status: %v", dberr)
		return nil
	}
	if status, ok := val.Value().(string); ok && status == "Playing" {
		if err := s.player.TogglePause(); err != nil {
			log.Printf("error toggling pause via MPRIS: %v", err)
		}
	}
	return nil
}

func (s *Server) Stop() *dbus.Error {
	log.Println("MPRIS: Stop called")
	s.player.Stop()
	return nil
}

func (s *Server) Quit() *dbus.Error {
	log.Println("MPRIS: Quit called")
	s.player.Stop()
	return nil
}

func (s *Server) Raise() *dbus.Error {
	return nil
}

func (s *Server) Shutdown() {
	s.conn.Close()
}
