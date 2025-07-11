package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MusicDirs []string   `yaml:"music_dirs"`
	Stations  []Stations `yaml:"stations"`
}

type Stations struct {
	Name string `yaml:"name"`
	Url  string `yaml:"url"`
}

func LoadConfig() (*Config, error) {
	cfgPath, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to load user config directory: %s", err)
	}

	cfgPath = filepath.Join(cfgPath + "/tunecli/config.yaml")

	_, err = os.Stat(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg, err := saveDefaultConfig(cfgPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create default config: %s", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to load config file: %s", err)
	}

	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %s", err)
	}

	var config Config
	err = yaml.Unmarshal(cfg, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %s", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %s", err)
	}

	for i, dir := range config.MusicDirs {
		if strings.HasPrefix(dir, "~/") {
			config.MusicDirs[i] = filepath.Join(home, dir[2:])
		}
	}

	return &config, nil
}

func saveDefaultConfig(cfgPath string) (*Config, error) {
	config := &Config{
		MusicDirs: []string{"~/Music"},
		Stations: []Stations{
			{
				Name: "Record Lo-Fi",
				Url:  "https://radiorecord.hostingradio.ru/lofi96.aacp",
			}, {
				Name: "Record Synthwave",
				Url:  "https://radiorecord.hostingradio.ru/synth96.aacp",
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %s", err)
	}

	if err = os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %s", err)
	}

	err = os.WriteFile(cfgPath, data, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write config file: %s", err)
	}

	return config, nil
}
