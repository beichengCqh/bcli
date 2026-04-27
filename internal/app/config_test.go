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
