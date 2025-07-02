package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const cfgDirName = "tunecli"

type AppConfig struct {
	General  GeneralSettings `yaml:"general"`
	Hotkeys  Hotkeys         `yaml:"hotkeys"`
	Stations []RadioStation  `yaml:"stations"`
}

type GeneralSettings struct {
	MusicDirs []string `yaml:"music_dirs"`
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

func LoadConfigs() (*AppConfig, error) {
	cfgDirPath, err := GetConfigDirPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine config directory: %w", err)
	}

	if err := ensureConfigDir(cfgDirPath); err != nil {
		return nil, fmt.Errorf("could not ensure config directory: %w", err)
	}

	cfg := &AppConfig{}

	if err := loadConfigFile(filepath.Join(cfgDirPath, "general.yml"), &cfg.General); err != nil {
		return nil, fmt.Errorf("failed to load general config: %w", err)
	}

	if err := loadConfigFile(filepath.Join(cfgDirPath, "hotkeys.yml"), &cfg.Hotkeys); err != nil {
		return nil, fmt.Errorf("failed to load hotkeys config: %w", err)
	}

	if err := loadConfigFile(filepath.Join(cfgDirPath, "stations.yml"), &cfg.Stations); err != nil {
		return nil, fmt.Errorf("failed to load stations config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *AppConfig) Validate() error {
	if err := c.General.Validate(); err != nil {
		return fmt.Errorf("general settings validation failed: %w", err)
	}

	if err := c.Hotkeys.Validate(); err != nil {
		return fmt.Errorf("hotkeys validation failed: %w", err)
	}

	for i, station := range c.Stations {
		if err := station.Validate(); err != nil {
			return fmt.Errorf("station %d validation failed: %w", i, err)
		}
	}

	return nil
}

func (g *GeneralSettings) Validate() error {
	if len(g.MusicDirs) == 0 {
		return fmt.Errorf("at least one music directory must be specified")
	}

	for _, dir := range g.MusicDirs {
		if strings.TrimSpace(dir) == "" {
			return fmt.Errorf("music directory path cannot be empty")
		}

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("music directory does not exist: %s", dir)
		}
	}

	return nil
}

func (h *Hotkeys) Validate() error {
	keys := map[string]string{
		"play_pause": h.PlayPause,
		"next":       h.Next,
		"previous":   h.Previous,
		"stop":       h.Stop,
	}

	for name, key := range keys {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("hotkey %s cannot be empty", name)
		}
	}

	return nil
}

func (r *RadioStation) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("station name cannot be empty")
	}

	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("station URL cannot be empty")
	}

	if _, err := url.Parse(r.URL); err != nil {
		return fmt.Errorf("invalid station URL: %w", err)
	}

	return nil
}

func ensureConfigDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("could not create config directory: %w", err)
		}
	}
	return nil
}

func loadConfigFile(path string, out any) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file %s: %w", path, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("config file is empty: %s", path)
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("could not unmarshal yaml in %s: %w", path, err)
	}

	return nil
}

func GetConfigDirPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not get user config directory: %w", err)
	}
	return filepath.Join(configHome, cfgDirName), nil
}
