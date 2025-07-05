package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AppName    = "tunecli"
	ConfigFile = "config.yml"
)

type Config struct {
	MusicDirs []string       `yaml:"music_dirs"`
	Stations  []RadioStation `yaml:"stations"`
	Keybinds  Keybinds       `yaml:"keybinds"`
	Theme     Theme          `yaml:"theme"`
	Player    Player         `yaml:"player"`
}

type RadioStation struct {
	Name string   `yaml:"name"`
	URL  string   `yaml:"url"`
	Tags []string `yaml:"tags,omitempty"`
}

type Keybinds struct {
	PlayPause  string `yaml:"play_pause"`
	Stop       string `yaml:"stop"`
	Next       string `yaml:"next"`
	Previous   string `yaml:"previous"`
	VolumeUp   string `yaml:"volume_up"`
	VolumeDown string `yaml:"volume_down"`
	Quit       string `yaml:"quit"`
	Switch     string `yaml:"switch"`
}

type Theme struct {
	PrimaryColor   string `yaml:"primary_color"`
	SecondaryColor string `yaml:"secondary_color"`
	PlayingColor   string `yaml:"playing_color"`
	BorderColor    string `yaml:"border_color"`
}

type Player struct {
	SocketPath string `yaml:"socket_path"`
	Volume     int    `yaml:"default_volume"`
}

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("get config path: %w", err)
	}

	if err := ensureConfigDir(filepath.Dir(configPath)); err != nil {
		return nil, fmt.Errorf("ensure config dir: %w", err)
	}

	var cfg Config

	if err := loadConfigFile(configPath, &cfg); err != nil {
		if os.IsNotExist(err) {
			if err := saveConfig(configPath, &cfg); err != nil {
				return nil, fmt.Errorf("save default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("load config: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.MusicDirs) == 0 {
		return fmt.Errorf("at least one music directory required")
	}

	for _, dir := range c.MusicDirs {
		if strings.TrimSpace(dir) == "" {
			return fmt.Errorf("empty music directory")
		}
	}

	if len(c.Stations) == 0 {
		return fmt.Errorf("at least one radio station required")
	}

	for i, station := range c.Stations {
		if strings.TrimSpace(station.Name) == "" {
			return fmt.Errorf("station %d: name required", i)
		}
		if strings.TrimSpace(station.URL) == "" {
			return fmt.Errorf("station %d: URL required", i)
		}
	}

	if c.Player.Volume < 0 || c.Player.Volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100")
	}

	return nil
}

func getConfigPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configHome, AppName, ConfigFile), nil
}

func ensureConfigDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}
	return nil
}

func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	return nil
}

func saveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
