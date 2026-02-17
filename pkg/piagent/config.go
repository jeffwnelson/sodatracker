package piagent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIURL    string       `yaml:"apiUrl"`
	APIKey    string       `yaml:"apiKey"`
	MachineID string       `yaml:"machineId"`
	PollMs    int          `yaml:"pollMs"`
	RetryMs   int          `yaml:"retryMs"`
	DBPath    string       `yaml:"dbPath"`
	Pins      []PinConfig  `yaml:"pins"`
}

type PinConfig struct {
	Pin      int    `yaml:"pin"`
	Slot     int    `yaml:"slot"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.PollMs == 0 {
		cfg.PollMs = 100
	}
	if cfg.RetryMs == 0 {
		cfg.RetryMs = 30000
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "piagent-cache.db"
	}
	return &cfg, nil
}
