package storage

import (
	"errors"
	"os"
	"path/filepath"
)

func ConfigWritePath() (string, error) {
	if path := os.Getenv("BCLI_CONFIG"); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bcli", "configs", "connections.json"), nil
}

func ConfigReadPath() (string, error) {
	if path := os.Getenv("BCLI_CONFIG"); path != "" {
		return path, nil
	}

	path, err := ConfigWritePath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return path, err
	}

	// 兼容早期版本的默认路径；保存时会写入新的 configs 目录。
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bcli", "config.json"), nil
}
