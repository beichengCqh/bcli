package tui

import (
	"strings"
	"testing"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeCredentialStore struct {
	values map[string]string
}

func (s *fakeCredentialStore) Set(kind string, name string, secret string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[kind+":"+auth.NormalizeProfileName(name)] = secret
	return nil
}

func (s *fakeCredentialStore) Get(kind string, name string) (string, error) {
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
}

func (s *memoryProfileStore) Load() (profile.Config, error) {
	return s.cfg, nil
}

func (s *memoryProfileStore) Save(cfg profile.Config) error {
	s.cfg = cfg
	return nil
}

func TestTUISaveFormStoresProfileAndCredential(t *testing.T) {
	credentials := &fakeCredentialStore{}
	profiles := &memoryProfileStore{}
	m, err := newModel(profile.NewService(profiles), auth.NewService(credentials))
	if err != nil {
		t.Fatalf("newModel returned error: %v", err)
	}

	m.screen = formScreen
	m.form = form{
		kind:     "redis",
		name:     "cache",
		host:     "127.0.0.1",
		port:     "6379",
		user:     "default",
		database: "1",
		password: "redis-secret",
	}
	m = m.saveForm()

	p, err := m.cfg.ExternalProfile("redis", "cache")
	if err != nil {
		t.Fatalf("ExternalProfile returned error: %v", err)
	}
	if p.Host != "127.0.0.1" || p.Port != 6379 || p.User != "default" || p.Database != "1" {
		t.Fatalf("profile = %#v", p)
	}
	if got := credentials.values["redis:cache"]; got != "redis-secret" {
		t.Fatalf("stored secret = %q", got)
	}
}

func TestNewFormAutoDetectsExecutableWhenAvailable(t *testing.T) {
	f := newForm(profile.Config{}, "mysql")
	if executable, ok := profile.DetectExecutable("mysql"); ok && f.executable != executable {
		t.Fatalf("executable = %q, want %q", f.executable, executable)
	}
}

func TestNewFormUsesConfiguredClientExecutable(t *testing.T) {
	cfg := profile.Config{}
	cfg.SetClient("mysql", profile.ClientConfig{Enabled: true, Executable: "/usr/bin/true"})
	f := newForm(cfg, "mysql")
	if f.executable != "/usr/bin/true" {
		t.Fatalf("executable = %q", f.executable)
	}
}

func TestTUITestConnectionUsesCurrentForm(t *testing.T) {
	credentials := &fakeCredentialStore{}
	profiles := &memoryProfileStore{}
	m, err := newModel(profile.NewService(profiles), auth.NewService(credentials))
	if err != nil {
		t.Fatalf("newModel returned error: %v", err)
	}

	m.screen = formScreen
	m.form = form{
		kind:       "mysql",
		name:       "local",
		executable: "/usr/bin/true",
		password:   "mysql-secret",
	}
	m = m.testConnection()

	if !strings.Contains(m.message, "test ok: mysql/local") {
		t.Fatalf("message = %q", m.message)
	}
}

func TestTUITestConnectionDoesNotLeakPasswordOnFailure(t *testing.T) {
	credentials := &fakeCredentialStore{}
	profiles := &memoryProfileStore{}
	m, err := newModel(profile.NewService(profiles), auth.NewService(credentials))
	if err != nil {
		t.Fatalf("newModel returned error: %v", err)
	}

	m.screen = formScreen
	m.form = form{
		kind:       "redis",
		name:       "cache",
		executable: "/usr/bin/false",
		password:   "redis-secret",
	}
	m = m.testConnection()

	if !strings.Contains(m.message, "connection test failed") {
		t.Fatalf("message = %q", m.message)
	}
	if strings.Contains(m.message, "redis-secret") {
		t.Fatalf("password leaked in message: %q", m.message)
	}
}

func TestTUIListQuitReturnsTeaQuit(t *testing.T) {
	m := model{}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("quit key did not return a command")
	}
}

func TestTUINavigationOpensProfilesAndTools(t *testing.T) {
	m := model{}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = next.(model)
	if m.screen != profilesScreen {
		t.Fatalf("screen = %v, want profilesScreen", m.screen)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = next.(model)
	if m.screen != toolsScreen {
		t.Fatalf("screen = %v, want toolsScreen", m.screen)
	}
}

func TestTUINavigationUsesLeftAndRightArrows(t *testing.T) {
	m := model{}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(model)
	if m.screen != profilesScreen {
		t.Fatalf("screen = %v, want profilesScreen", m.screen)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(model)
	if m.screen != toolsScreen {
		t.Fatalf("screen = %v, want toolsScreen", m.screen)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(model)
	if m.screen != profilesScreen {
		t.Fatalf("screen = %v, want profilesScreen", m.screen)
	}
}

func TestTUIToolsBase64Encode(t *testing.T) {
	m := model{screen: toolsScreen}
	for i, action := range toolActions {
		if action.id == "base64encode" {
			m.tool.selected = i
			break
		}
	}
	m.tool.input = "hello"

	m = m.runTool()
	if m.tool.output != "aGVsbG8=" {
		t.Fatalf("output = %q, want aGVsbG8=", m.tool.output)
	}
}

func TestTUIToolsInputKeepsDigits(t *testing.T) {
	m := model{screen: toolsScreen}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = next.(model)

	if m.screen != toolsScreen {
		t.Fatalf("screen = %v, want toolsScreen", m.screen)
	}
	if m.tool.input != "2" {
		t.Fatalf("input = %q, want 2", m.tool.input)
	}
}

func TestTUIFormAllowsSpacesOutsideTypeField(t *testing.T) {
	m := model{screen: formScreen, form: form{field: 7, kind: "mysql", args: "--ssl"}}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(model)

	if m.form.args != "--ssl " {
		t.Fatalf("args = %q, want trailing space", m.form.args)
	}
}
