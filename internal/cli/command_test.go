package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeCredentialStore struct {
	values map[string]string
	err    error
}

func (s *fakeCredentialStore) Set(kind string, name string, secret string) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[kind+":"+auth.NormalizeProfileName(name)] = secret
	return nil
}

func (s *fakeCredentialStore) Get(kind string, name string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	value, ok := s.values[kind+":"+auth.NormalizeProfileName(name)]
	if !ok {
		return "", auth.ErrCredentialNotFound
	}
	return value, nil
}

func (s *fakeCredentialStore) Delete(kind string, name string) error {
	delete(s.values, kind+":"+auth.NormalizeProfileName(name))
	return nil
}

type memoryProfileStore struct {
	cfg profile.Config
	err error
}

func (s *memoryProfileStore) Load() (profile.Config, error) {
	if s.err != nil {
		return profile.Config{}, s.err
	}
	return s.cfg, nil
}

func (s *memoryProfileStore) Save(cfg profile.Config) error {
	if s.err != nil {
		return s.err
	}
	s.cfg = cfg
	return nil
}

func TestParseProfileArgs(t *testing.T) {
	selected, rest, err := parseProfileArgs([]string{"--profile", "local", "--", "-e", "select 1"})
	if err != nil {
		t.Fatalf("parseProfileArgs returned error: %v", err)
	}
	if selected != "local" {
		t.Fatalf("profile = %q, want local", selected)
	}
	if strings.Join(rest, " ") != "-e select 1" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestToolsBase64Encode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"tools", "base64", "encode", "hello"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "aGVsbG8=" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestToolsURLDecode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"tools", "urldecode", "a%20b"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "a b" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExternalAuthStoresCredential(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := &fakeCredentialStore{}
	profiles := &memoryProfileStore{cfg: profile.Config{
		Redis: map[string]profile.ExternalProfile{
			"default": {Executable: "nonexistent-bcli-test-client"},
		},
	}}
	r := NewRunner(strings.NewReader(""), &stdout, &stderr, store, profiles)

	code := r.Run([]string{"auth", "redis", "--profile", "cache", "secret-value"})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if store.values["redis:cache"] != "secret-value" {
		t.Fatalf("stored secret = %q", store.values["redis:cache"])
	}
	if strings.Contains(stdout.String(), "secret-value") || strings.Contains(stderr.String(), "secret-value") {
		t.Fatalf("secret leaked in output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestOldNestedAuthCommandIsNotAccepted(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := &fakeCredentialStore{}
	r := NewRunner(strings.NewReader(""), &stdout, &stderr, store, &memoryProfileStore{})

	code := r.Run([]string{"redis", "auth", "--profile", "cache", "secret-value"})
	if code == 0 {
		t.Fatalf("old nested auth command unexpectedly succeeded")
	}
	if got := store.values["redis:cache"]; got != "" {
		t.Fatalf("old nested auth stored secret = %q", got)
	}
}

func TestProfileSetAndListJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := &fakeCredentialStore{}
	profiles := &memoryProfileStore{}
	r := NewRunner(strings.NewReader(""), &stdout, &stderr, store, profiles)

	code := r.Run([]string{"profile", "set", "mysql", "local", "--host", "127.0.0.1", "--port", "3306", "--user", "root", "--database", "app"})
	if code != 0 {
		t.Fatalf("set code = %d, stderr = %s", code, stderr.String())
	}
	stdout.Reset()

	code = r.Run([]string{"profile", "list", "--json"})
	if code != 0 {
		t.Fatalf("list code = %d, stderr = %s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, `"kind": "mysql"`) || !strings.Contains(output, `"name": "local"`) {
		t.Fatalf("profile list json missing profile: %s", output)
	}
	if strings.Contains(output, "password") || strings.Contains(output, "secret") {
		t.Fatalf("profile list json leaked credential-looking content: %s", output)
	}
}

func TestApplyInitModelStoresSelectedClients(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	profiles := &memoryProfileStore{}
	r := NewRunner(strings.NewReader(""), &stdout, &stderr, &fakeCredentialStore{}, profiles)
	m := initModel{options: []initClientOption{
		{kind: "mysql", selected: true, executable: "/bin/mysql"},
		{kind: "redis", selected: false},
	}}

	if err := r.applyInitModel(m, func(io.Reader, io.Writer, io.Writer, string, ...string) error {
		t.Fatalf("installer should not run")
		return nil
	}); err != nil {
		t.Fatalf("applyInitModel returned error: %v", err)
	}
	mysql := profiles.cfg.Client("mysql")
	if !mysql.Enabled {
		t.Fatalf("mysql client was not enabled")
	}
	if mysql.Executable != "/bin/mysql" {
		t.Fatalf("mysql executable = %q", mysql.Executable)
	}
	redis := profiles.cfg.Client("redis")
	if redis.Enabled {
		t.Fatalf("redis client unexpectedly enabled")
	}
	if !strings.Contains(stdout.String(), "bcli init completed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitModelUsesMultiSelectAndInstallScreen(t *testing.T) {
	m := initModel{options: []initClientOption{
		{kind: "mysql", title: "MySQL client"},
		{kind: "redis", title: "Redis client"},
	}}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(initModel)
	if !m.options[0].selected {
		t.Fatalf("mysql option was not selected")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(initModel)
	if m.screen != initSelectInstalls {
		t.Fatalf("screen = %v, want install selection", m.screen)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(initModel)
	if !m.options[0].install {
		t.Fatalf("mysql install option was not selected")
	}
}

func TestInitHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	r := NewRunner(strings.NewReader(""), &stdout, &stderr, &fakeCredentialStore{}, &memoryProfileStore{})

	code := r.Run([]string{"init", "help"})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "bcli init") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
