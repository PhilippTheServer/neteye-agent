package config

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for neteye-agent.
type Config struct {
	CenterURL       string        `yaml:"center_url"`        // ws://host:9090/ws
	Hostname        string        `yaml:"hostname"`          // override auto-detect
	CollectInterval time.Duration `yaml:"collect_interval"`  // default 5s
	ReconnectDelay  time.Duration `yaml:"reconnect_delay"`   // default 5s
	AgentVersion    string        `yaml:"agent_version"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		CenterURL:       "ws://localhost:9090/ws",
		Hostname:        hostname,
		CollectInterval: 5 * time.Second,
		ReconnectDelay:  5 * time.Second,
		AgentVersion:    "1.0.0",
	}
}

// Load reads a YAML config file and overlays environment variables.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open config %s: %w", path, err)
		}
		defer f.Close()
		if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
			return nil, fmt.Errorf("decode config: %w", err)
		}
	}

	if v := os.Getenv("NETEYE_CENTER"); v != "" {
		cfg.CenterURL = v
	}
	if v := os.Getenv("NETEYE_HOSTNAME"); v != "" {
		cfg.Hostname = v
	}
	if cfg.Hostname == "" {
		cfg.Hostname, _ = os.Hostname()
	}

	return cfg, nil
}

// OS returns the current operating system string.
func OS() string { return runtime.GOOS }

// Arch returns the current architecture string.
func Arch() string { return runtime.GOARCH }
