package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	MySQL map[string]ExternalProfile `json:"mysql"`
	Redis map[string]ExternalProfile `json:"redis"`
}

type ExternalProfile struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func configPath() (string, error) {
	if path := os.Getenv("BCLI_CONFIG"); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bcli", "config.json"), nil
}

func (c Config) ExternalProfile(kind string, name string) (ExternalProfile, error) {
	profiles := c.MySQL
	if kind == "redis" {
		profiles = c.Redis
	}

	if name == "" {
		name = "default"
	}

	if len(profiles) == 0 {
		if name == "default" {
			return ExternalProfile{}, nil
		}
		return ExternalProfile{}, fmt.Errorf("%s profile %q not found", kind, name)
	}

	profile, ok := profiles[name]
	if !ok {
		return ExternalProfile{}, fmt.Errorf("%s profile %q not found", kind, name)
	}
	return profile, nil
}
