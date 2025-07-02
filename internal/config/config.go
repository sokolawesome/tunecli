package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	dirName  = "tunecli"
	fileName = "config.yml"
)

type Config struct {
	MusicDirs []string       `yaml:"music_dirs"`
	Hotkeys   Hotkeys        `yaml:"hotkeys"`
	Stations  []RadioStation `yaml:"stations"`
}

type Hotkeys struct {
	PlayPause string `yaml:"play_pause"`
	Next      string `yaml:"next"`
	Previous  string `yaml:"previous"`
	Stop      string `yaml:"stop"`
}

type RadioStation struct {
	Name string   `yaml:"name"`
	URL  string   `yaml:"url"`
	Tags []string `yaml:"tags"`
}

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("get config path: %w", err)
	}

	if err := ensureConfigDir(filepath.Dir(configPath)); err != nil {
		return nil, fmt.Errorf("ensure config dir: %w", err)
	}

	cfg := &Config{}
	if err := loadConfigFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if len(c.MusicDirs) == 0 {
		return fmt.Errorf("at least one music directory required")
	}

	for _, dir := range c.MusicDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return fmt.Errorf("empty music directory")
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("music directory not found: %s", dir)
		}
	}

	if err := c.Hotkeys.validate(); err != nil {
		return fmt.Errorf("hotkeys: %w", err)
	}

	for i, station := range c.Stations {
		if err := station.validate(); err != nil {
			return fmt.Errorf("station %d: %w", i, err)
		}
	}

	return nil
}

func (h *Hotkeys) validate() error {
	keys := map[string]string{
		"play_pause": h.PlayPause,
		"next":       h.Next,
		"previous":   h.Previous,
		"stop":       h.Stop,
	}

	for name, key := range keys {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s cannot be empty", name)
		}
	}

	return nil
}

func (r *RadioStation) validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	if _, err := url.Parse(r.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return nil
}

func getConfigPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configHome, dirName, fileName), nil
}

func ensureConfigDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}
	return nil
}

func loadConfigFile(path string, out *Config) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("config file is empty")
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	return nil
}
