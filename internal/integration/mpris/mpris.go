package mpris

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
	"github.com/sokolawesome/tunecli/internal/player"
)

const (
	interfaceName = "org.mpris.MediaPlayer2.Player"
	rootInterface = "org.mpris.MediaPlayer2"
	busName       = "org.mpris.MediaPlayer2.tunecli"
	objectPath    = "/org/mpris/MediaPlayer2"
)

type Server struct {
	conn   *dbus.Conn
	player *player.Player
	props  *prop.Properties
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

func NewServer(p *player.Player) (*Server, error) {
	if p == nil {
		return nil, fmt.Errorf("player cannot be nil")
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("could not connect to D-Bus session bus: %w", err)
	}

	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not request bus name: %w", err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, fmt.Errorf("bus name %s is already taken", busName)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		conn:   conn,
		player: p,
		ctx:    ctx,
		cancel: cancel,
	}

	if err := s.setupProperties(); err != nil {
		s.cleanup()
		return nil, fmt.Errorf("failed to setup properties: %w", err)
	}

	if err := s.exportInterfaces(); err != nil {
		s.cleanup()
		return nil, fmt.Errorf("failed to export interfaces: %w", err)
	}

	s.wg.Add(1)
	go s.watchPlayerState()

	log.Println("MPRIS server started successfully")
	return s, nil
}

func (s *Server) setupProperties() error {
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
			"Volume": {
				Value:    1.0,
				Writable: true,
				Emit:     prop.EmitTrue,
			},
			"Position": {
				Value:    int64(0),
				Writable: false,
				Emit:     prop.EmitFalse,
			},
			"CanPlay":    {Value: true, Writable: false, Emit: prop.EmitInvalidates},
			"CanPause":   {Value: true, Writable: false, Emit: prop.EmitInvalidates},
			"CanSeek":    {Value: true, Writable: false, Emit: prop.EmitInvalidates},
			"CanControl": {Value: true, Writable: false, Emit: prop.EmitInvalidates},
		},
		rootInterface: {
			"Identity": {
				Value:    "TuneCLI",
				Writable: false,
				Emit:     prop.EmitInvalidates,
			},
			"CanQuit": {
				Value:    true,
				Writable: false,
				Emit:     prop.EmitInvalidates,
			},
			"CanRaise": {
				Value:    false,
				Writable: false,
				Emit:     prop.EmitInvalidates,
			},
		},
	}

	var err error
	s.props, err = prop.Export(s.conn, objectPath, propsSpec)
	return err
}

func (s *Server) exportInterfaces() error {
	if err := s.conn.Export(s, objectPath, interfaceName); err != nil {
		return fmt.Errorf("failed to export player interface: %w", err)
	}

	if err := s.conn.Export(s, objectPath, rootInterface); err != nil {
		return fmt.Errorf("failed to export root interface: %w", err)
	}

	return nil
}

func (s *Server) watchPlayerState() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case state, ok := <-s.player.StateChanges:
			if !ok {
				return
			}
			s.updateMPRISState(state)
		}
	}
}

func (s *Server) updateMPRISState(state player.State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := "Stopped"
	if state.Title != "" {
		if state.IsPlaying {
			status = "Playing"
		} else {
			status = "Paused"
		}
	}

	if err := s.props.Set(interfaceName, "PlaybackStatus", dbus.MakeVariant(status)); err != nil {
		log.Printf("MPRIS: failed to set PlaybackStatus: %v", err)
	}

	metadata := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant("/org/mpris/MediaPlayer2/track/0"),
		"mpris:length":  dbus.MakeVariant(int64(state.Duration * 1000000)),
		"xesam:title":   dbus.MakeVariant(state.Title),
		"xesam:artist":  dbus.MakeVariant([]string{}),
		"xesam:album":   dbus.MakeVariant(""),
	}

	if err := s.props.Set(interfaceName, "Metadata", dbus.MakeVariant(metadata)); err != nil {
		log.Printf("MPRIS: failed to set Metadata: %v", err)
	}

	volume := float64(state.Volume) / 100.0
	if err := s.props.Set(interfaceName, "Volume", dbus.MakeVariant(volume)); err != nil {
		log.Printf("MPRIS: failed to set Volume: %v", err)
	}
}

func (s *Server) PlayPause() *dbus.Error {
	log.Println("MPRIS: PlayPause called")
	if err := s.player.TogglePause(); err != nil {
		log.Printf("MPRIS: PlayPause failed: %v", err)
		return dbus.NewError("org.mpris.MediaPlayer2.Player.Failed", []interface{}{err.Error()})
	}
	return nil
}

func (s *Server) Play() *dbus.Error {
	log.Println("MPRIS: Play called")

	val, err := s.props.Get(interfaceName, "PlaybackStatus")
	if err != nil {
		log.Printf("MPRIS: could not get playback status: %v", err)
		return nil
	}

	if status, ok := val.Value().(string); ok && status == "Paused" {
		if err := s.player.TogglePause(); err != nil {
			log.Printf("MPRIS: Play failed: %v", err)
			return dbus.NewError("org.mpris.MediaPlayer2.Player.Failed", []interface{}{err.Error()})
		}
	}
	return nil
}

func (s *Server) Pause() *dbus.Error {
	log.Println("MPRIS: Pause called")

	val, err := s.props.Get(interfaceName, "PlaybackStatus")
	if err != nil {
		log.Printf("MPRIS: could not get playback status: %v", err)
		return nil
	}

	if status, ok := val.Value().(string); ok && status == "Playing" {
		if err := s.player.TogglePause(); err != nil {
			log.Printf("MPRIS: Pause failed: %v", err)
			return dbus.NewError("org.mpris.MediaPlayer2.Player.Failed", []interface{}{err.Error()})
		}
	}
	return nil
}

func (s *Server) Stop() *dbus.Error {
	log.Println("MPRIS: Stop called")
	if err := s.player.Stop(); err != nil {
		log.Printf("MPRIS: Stop failed: %v", err)
		return dbus.NewError("org.mpris.MediaPlayer2.Player.Failed", []interface{}{err.Error()})
	}
	return nil
}

func (s *Server) Seek(offset int64) *dbus.Error {
	log.Printf("MPRIS: Seek called with offset: %d", offset)
	seconds := float64(offset) / 1000000.0
	if err := s.player.Seek(seconds); err != nil {
		log.Printf("MPRIS: Seek failed: %v", err)
		return dbus.NewError("org.mpris.MediaPlayer2.Player.Failed", []interface{}{err.Error()})
	}
	return nil
}

func (s *Server) SetPosition(trackID dbus.ObjectPath, position int64) *dbus.Error {
	log.Printf("MPRIS: SetPosition called with position: %d", position)
	seconds := float64(position) / 1000000.0
	if err := s.player.Seek(seconds); err != nil {
		log.Printf("MPRIS: SetPosition failed: %v", err)
		return dbus.NewError("org.mpris.MediaPlayer2.Player.Failed", []interface{}{err.Error()})
	}
	return nil
}

func (s *Server) Quit() *dbus.Error {
	log.Println("MPRIS: Quit called")
	if err := s.player.Stop(); err != nil {
		log.Printf("MPRIS: Quit failed: %v", err)
	}
	return nil
}

func (s *Server) Raise() *dbus.Error {
	log.Println("MPRIS: Raise called (no-op)")
	return nil
}

func (s *Server) cleanup() {
	if s.conn != nil {
		s.conn.Close()
	}
}

func (s *Server) Shutdown() {
	log.Println("MPRIS: Shutting down...")
	s.cancel()
	s.wg.Wait()
	s.cleanup()
	log.Println("MPRIS: Shutdown complete")
}
