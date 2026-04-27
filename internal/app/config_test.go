package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExternalProfileCommandArgs(t *testing.T) {
	mysql := ExternalProfile{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Database: "app",
		Args:     []string{"--protocol", "tcp"},
	}
	if got, want := mysql.CommandArgs("mysql"), []string{"-h", "127.0.0.1", "-P", "3306", "-u", "root", "--protocol", "tcp"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mysql args = %#v, want %#v", got, want)
	}

	redis := ExternalProfile{
		Host:     "127.0.0.1",
		Port:     6379,
		User:     "default",
		Database: "1",
		Args:     []string{"--tls"},
	}
	if got, want := redis.CommandArgs("redis"), []string{"-h", "127.0.0.1", "-p", "6379", "--user", "default", "-n", "1", "--tls"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("redis args = %#v, want %#v", got, want)
	}
}

func TestSaveConfigKeepsSecretsOutOfFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("BCLI_CONFIG", path)

	cfg := Config{}
	cfg.SetExternalProfile("mysql", "local", ExternalProfile{
		Host: "127.0.0.1",
		Port: 3306,
		User: "root",
	})
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
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

	path, err := configWritePath()
	if err != nil {
		t.Fatalf("configWritePath returned error: %v", err)
	}
	want := filepath.Join(home, ".bcli", "configs", "connections.json")
	if path != want {
		t.Fatalf("configWritePath = %q, want %q", path, want)
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

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	profile, err := cfg.ExternalProfile("mysql", "local")
	if err != nil {
		t.Fatalf("ExternalProfile returned error: %v", err)
	}
	if profile.Host != "127.0.0.1" {
		t.Fatalf("profile host = %q", profile.Host)
	}
}
