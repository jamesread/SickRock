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
		Port:      coalesce(k.String("port"), os.Getenv("PORT"), "8080"),
		LogLevel:  coalesce(k.String("log_level"), os.Getenv("LOG_LEVEL"), "info"),
		LogFormat: coalesce(k.String("log_format"), os.Getenv("LOG_FORMAT"), "text"),
		ConfigDir: dir,
	}
	return cfg, nil
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
