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
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
	Host       string   `json:"host,omitempty"`
	Port       int      `json:"port,omitempty"`
	User       string   `json:"user,omitempty"`
	Database   string   `json:"database,omitempty"`
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

func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
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

func (c *Config) ensureProfiles(kind string) map[string]ExternalProfile {
	if kind == "redis" {
		if c.Redis == nil {
			c.Redis = map[string]ExternalProfile{}
		}
		return c.Redis
	}
	if c.MySQL == nil {
		c.MySQL = map[string]ExternalProfile{}
	}
	return c.MySQL
}

func (c *Config) SetExternalProfile(kind string, name string, profile ExternalProfile) {
	c.ensureProfiles(kind)[normalizeProfileName(name)] = profile
}

func (c *Config) DeleteExternalProfile(kind string, name string) {
	delete(c.ensureProfiles(kind), normalizeProfileName(name))
}

func (p ExternalProfile) CommandArgs(kind string) []string {
	args := make([]string, 0, len(p.Args)+8)
	switch kind {
	case "mysql":
		if p.Host != "" {
			args = append(args, "-h", p.Host)
		}
		if p.Port != 0 {
			args = append(args, "-P", fmt.Sprintf("%d", p.Port))
		}
		if p.User != "" {
			args = append(args, "-u", p.User)
		}
		args = append(args, p.Args...)
	case "redis":
		if p.Host != "" {
			args = append(args, "-h", p.Host)
		}
		if p.Port != 0 {
			args = append(args, "-p", fmt.Sprintf("%d", p.Port))
		}
		if p.User != "" {
			args = append(args, "--user", p.User)
		}
		if p.Database != "" {
			args = append(args, "-n", p.Database)
		}
		args = append(args, p.Args...)
	default:
		args = append(args, p.Args...)
	}
	return args
}
