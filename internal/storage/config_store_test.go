package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bcli/internal/core/profile"
)

func TestSaveConfigKeepsSecretsOutOfFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("BCLI_CONFIG", path)

	cfg := profile.Config{}
	cfg.SetExternalProfile("mysql", "local", profile.ExternalProfile{
		Host: "127.0.0.1",
		Port: 3306,
		User: "root",
	})
	if err := (ConfigStore{}).Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("saved config is not valid JSON: %v", err)
	}
	if string(data) == "" || strings.Contains(string(data), "password") || strings.Contains(string(data), "secret") {
		t.Fatalf("saved config contains credential-looking content: %s", string(data))
	}
}

func TestDefaultConfigPathUsesConfigsDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BCLI_CONFIG", "")

	path, err := ConfigWritePath()
	if err != nil {
		t.Fatalf("ConfigWritePath returned error: %v", err)
	}
	want := filepath.Join(home, ".bcli", "configs", "connections.json")
	if path != want {
		t.Fatalf("ConfigWritePath = %q, want %q", path, want)
	}
}

func TestLoadConfigFallsBackToLegacyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BCLI_CONFIG", "")

	legacyPath := filepath.Join(home, ".bcli", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"mysql":{"local":{"host":"127.0.0.1"}}}`), 0600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := (ConfigStore{}).Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	p, err := cfg.ExternalProfile("mysql", "local")
	if err != nil {
		t.Fatalf("ExternalProfile returned error: %v", err)
	}
	if p.Host != "127.0.0.1" {
		t.Fatalf("profile host = %q", p.Host)
	}
}
