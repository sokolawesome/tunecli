package main

import (
	"log"
	"time"

	"github.com/sokolawesome/tunecli/internal/config"
	"github.com/sokolawesome/tunecli/internal/player"
)

func main() {
	log.Println("loading configuration files...")
	cfg, err := config.LoadConfigs()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Println("starting MPV player...")
	p, err := player.NewPlayer()
	if err != nil {
		log.Fatalf("failed to start player: %v", err)
	}
	defer p.Shutdown()

	if len(cfg.Stations) > 0 {
		station := cfg.Stations[0]
		log.Printf("Playing first station: %s (%s)", station.Name, station.URL)
		if err := p.LoadFile(station.URL, "replace"); err != nil {
			log.Fatalf("Failed to load file: %v", err)
		}
	}

	log.Println("Playback started. App will close in 15 seconds.")
	time.Sleep(15 * time.Second)

	log.Println("Pausing...")
	p.TogglePause()
	time.Sleep(3 * time.Second)

	log.Println("Resuming...")
	p.TogglePause()
	time.Sleep(5 * time.Second)

	log.Println("Shutting down.")
}
