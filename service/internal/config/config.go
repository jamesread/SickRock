package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	log "github.com/sirupsen/logrus"
)

const envPrefix = "SICKROCK_"

// Config holds application configuration loaded via koanf.
type Config struct {
	Port      string
	LogLevel  string
	LogFormat string
	ConfigDir string
}

// Load builds configuration from -configdir, config file, and environment.
// Integration tests can pass -configdir to point at a directory containing config.yaml and env.
func Load() (*Config, error) {
	configDir := flag.String("configdir", "", "Directory containing config.yaml (and optional SickRock.env)")
	flag.Parse()

	k := koanf.New(".")

	if err := k.Load(env.Provider(envPrefix, ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, envPrefix))
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env config: %w", err)
	}

	dir := *configDir
	if dir == "" {
		dir = os.Getenv(envPrefix + "CONFIGDIR")
	}
	if dir != "" {
		configPath := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("loading config file: %w", err)
			}
		}
	}

	cfg := &Config{
		Port: coalesceString("port", []stringCandidate{
			{`koanf key "port" (SICKROCK_PORT env and/or config.yaml, env loaded before file)`, k.String("listen_port")},
			{"PORT env", os.Getenv("PORT")},
			{"default", "8080"},
		}),
		LogLevel: coalesceString("log_level", []stringCandidate{
			{`koanf key "log_level" (SICKROCK_LOG_LEVEL env and/or config.yaml, env loaded before file)`, k.String("log_level")},
			{"LOG_LEVEL env", os.Getenv("LOG_LEVEL")},
			{"default", "info"},
		}),
		LogFormat: coalesceString("log_format", []stringCandidate{
			{`koanf key "log_format" (SICKROCK_LOG_FORMAT env and/or config.yaml, env loaded before file)`, k.String("log_format")},
			{"LOG_FORMAT env", os.Getenv("LOG_FORMAT")},
			{"default", "text"},
		}),
		ConfigDir: dir,
	}
	return cfg, nil
}

type stringCandidate struct {
	source string
	value  string
}

// coalesceString returns the first non-empty candidate.value and logs which source won.
func coalesceString(key string, candidates []stringCandidate) string {
	for _, c := range candidates {
		if c.value != "" {
			log.WithFields(log.Fields{
				"config_key":    key,
				"config_source": c.source,
				"config_value":  c.value,
			}).Info("resolved config setting")
			return c.value
		}
	}
	return ""
}
