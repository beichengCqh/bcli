package profile

import (
	"fmt"

	"bcli/internal/core/auth"
)

type Config struct {
	MySQL   map[string]ExternalProfile `json:"mysql"`
	Redis   map[string]ExternalProfile `json:"redis"`
	Clients map[string]ClientConfig    `json:"clients,omitempty"`
}

type ExternalProfile struct {
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
	Host       string   `json:"host,omitempty"`
	Port       int      `json:"port,omitempty"`
	User       string   `json:"user,omitempty"`
	Database   string   `json:"database,omitempty"`
}

type ClientConfig struct {
	Enabled    bool   `json:"enabled"`
	Executable string `json:"executable,omitempty"`
}

func NormalizeName(name string) string {
	return auth.NormalizeProfileName(name)
}

func (c Config) ExternalProfile(kind string, name string) (ExternalProfile, error) {
	profiles := c.MySQL
	if kind == "redis" {
		profiles = c.Redis
	}

	name = NormalizeName(name)
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

func (c *Config) EnsureProfiles(kind string) map[string]ExternalProfile {
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

func (c *Config) SetExternalProfile(kind string, name string, p ExternalProfile) {
	c.EnsureProfiles(kind)[NormalizeName(name)] = p
}

func (c *Config) DeleteExternalProfile(kind string, name string) {
	delete(c.EnsureProfiles(kind), NormalizeName(name))
}

func (c Config) Client(kind string) ClientConfig {
	if c.Clients == nil {
		return ClientConfig{}
	}
	return c.Clients[kind]
}

func (c *Config) SetClient(kind string, client ClientConfig) {
	if c.Clients == nil {
		c.Clients = map[string]ClientConfig{}
	}
	c.Clients[kind] = client
}

func (c Config) HasClientConfig() bool {
	return len(c.Clients) > 0
}

func IsSupportedKind(kind string) bool {
	return kind == "mysql" || kind == "redis"
}
