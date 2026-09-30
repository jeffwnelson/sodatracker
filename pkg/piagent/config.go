package piagent

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIURL    string `yaml:"apiUrl"`
	APIKey    string `yaml:"apiKey"`
	MachineID string `yaml:"machineId"`
	PollMs    int    `yaml:"pollMs"`
	RetryMs   int    `yaml:"retryMs"`
	// A contact must read closed this long before it counts as a vend, and
	// then read open this long before the vend is reported.
	MinClosedMs int         `yaml:"minClosedMs"`
	MinOpenMs   int         `yaml:"minOpenMs"`
	DBPath      string      `yaml:"dbPath"`
	Pins        []PinConfig `yaml:"pins"`
}

type PinConfig struct {
	Pin  int `yaml:"pin"`
	Slot int `yaml:"slot"`
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
	applyEnvOverrides(&cfg)
	if cfg.PollMs == 0 {
		cfg.PollMs = 100
	}
	if cfg.RetryMs == 0 {
		cfg.RetryMs = 30000
	}
	if cfg.MinClosedMs == 0 {
		cfg.MinClosedMs = 100
	}
	if cfg.MinOpenMs == 0 {
		cfg.MinOpenMs = 500
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "piagent-cache.db"
	}
	return &cfg, nil
}

// applyEnvOverrides lets a container override deployment-specific settings
// without rewriting the mounted YAML. Pin mappings stay YAML-only.
func applyEnvOverrides(cfg *Config) {
	overrideString(&cfg.APIURL, "PIAGENT_API_URL")
	overrideString(&cfg.APIKey, "PIAGENT_API_KEY")
	overrideString(&cfg.MachineID, "PIAGENT_MACHINE_ID")
	overrideString(&cfg.DBPath, "PIAGENT_DB_PATH")
	overrideInt(&cfg.PollMs, "PIAGENT_POLL_MS")
	overrideInt(&cfg.RetryMs, "PIAGENT_RETRY_MS")
	overrideInt(&cfg.MinClosedMs, "PIAGENT_MIN_CLOSED_MS")
	overrideInt(&cfg.MinOpenMs, "PIAGENT_MIN_OPEN_MS")
}

func overrideString(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func overrideInt(dst *int, key string) {
	v := os.Getenv(key)
	if v == "" {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return
	}
	*dst = n
}
