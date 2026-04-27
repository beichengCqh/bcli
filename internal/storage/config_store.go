package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"bcli/internal/core/profile"
)

type ConfigStore struct{}

func (ConfigStore) Load() (profile.Config, error) {
	path, err := ConfigReadPath()
	if err != nil {
		return profile.Config{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return profile.Config{}, nil
	}
	if err != nil {
		return profile.Config{}, err
	}

	var cfg profile.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return profile.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func (ConfigStore) Save(cfg profile.Config) error {
	path, err := ConfigWritePath()
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
