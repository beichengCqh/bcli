package app

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUISaveFormStoresProfileAndCredential(t *testing.T) {
	t.Setenv("BCLI_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	store := &fakeCredentialStore{}
	model, err := newTUIModel(store)
	if err != nil {
		t.Fatalf("newTUIModel returned error: %v", err)
	}

	model.screen = tuiFormScreen
	model.form = tuiForm{
		kind:     "redis",
		name:     "cache",
		host:     "127.0.0.1",
		port:     "6379",
		user:     "default",
		database: "1",
		password: "redis-secret",
	}
	model = model.saveForm()

	profile, err := model.cfg.ExternalProfile("redis", "cache")
	if err != nil {
		t.Fatalf("ExternalProfile returned error: %v", err)
	}
	if profile.Host != "127.0.0.1" || profile.Port != 6379 || profile.User != "default" || profile.Database != "1" {
		t.Fatalf("profile = %#v", profile)
	}
	if got := store.values["redis:cache"]; got != "redis-secret" {
		t.Fatalf("stored secret = %q", got)
	}
}

func TestTUIListQuitReturnsTeaQuit(t *testing.T) {
	model := tuiModel{}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("quit key did not return a command")
	}
}
