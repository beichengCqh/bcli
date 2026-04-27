package profile

import (
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

func TestInstallHintUsesKind(t *testing.T) {
	if !strings.Contains(InstallHint("mysql"), "mysql") {
		t.Fatalf("mysql install hint = %q", InstallHint("mysql"))
	}
	if !strings.Contains(InstallHint("redis"), "redis") {
		t.Fatalf("redis install hint = %q", InstallHint("redis"))
	}
}

func TestResolveExecutableKeepsConfiguredPathWhenMissing(t *testing.T) {
	path, ok := ResolveExecutable("mysql", "/missing/bcli-test/mysql")
	if ok {
		t.Fatalf("missing configured path unexpectedly resolved")
	}
	if path != "/missing/bcli-test/mysql" {
		t.Fatalf("path = %q", path)
	}
}

func TestClientConfig(t *testing.T) {
	var cfg Config
	if cfg.HasClientConfig() {
		t.Fatalf("empty config unexpectedly has client config")
	}
	cfg.SetClient("mysql", ClientConfig{Enabled: true, Executable: "/bin/mysql"})
	if !cfg.HasClientConfig() {
		t.Fatalf("config should have client config")
	}
	client := cfg.Client("mysql")
	if !client.Enabled || client.Executable != "/bin/mysql" {
		t.Fatalf("client = %#v", client)
	}
}
