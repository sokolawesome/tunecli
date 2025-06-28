package config

import (
	"fmt"
	"os"
	"path/filepath"

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
	var cfg AppConfig

	cfgDirPath, err := GetConfigDirPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine config directory: %w", err)
	}

	if err := loadFile(filepath.Join(cfgDirPath, "general.yml"), &cfg.General); err != nil {
		return nil, err
	}
	if err := loadFile(filepath.Join(cfgDirPath, "hotkeys.yml"), &cfg.Hotkeys); err != nil {
		return nil, err
	}
	if err := loadFile(filepath.Join(cfgDirPath, "stations.yml"), &cfg.Stations); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func loadFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file %s: %w", path, err)
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
