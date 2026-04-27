package external

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
)

type fakeCredentialStore struct {
	values map[string]string
}

func TestConnectionTestArgs(t *testing.T) {
	mysql := profile.ExternalProfile{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Database: "app",
	}
	if got, want := testArgs("mysql", mysql), []string{"-h", "127.0.0.1", "-P", "3306", "-u", "root", "-e", "select 1", "app"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mysql test args = %#v, want %#v", got, want)
	}

	redis := profile.ExternalProfile{
		Host: "127.0.0.1",
		Port: 6379,
	}
	if got, want := testArgs("redis", redis), []string{"-h", "127.0.0.1", "-p", "6379", "ping"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("redis test args = %#v, want %#v", got, want)
	}
}

func TestConnectionTestReturnsInstallHintWhenExecutableMissing(t *testing.T) {
	var stderr bytes.Buffer
	s := NewService(profile.NewService(&memoryProfileStore{}), auth.NewService(&fakeCredentialStore{}), strings.NewReader(""), &bytes.Buffer{}, &stderr)
	err := s.TestConnection("mysql", "local", profile.ExternalProfile{Executable: "/missing/bcli-test/mysql"}, "")
	if err == nil {
		t.Fatalf("TestConnection returned nil")
	}
	if !strings.Contains(err.Error(), "mysql client not found") {
		t.Fatalf("error = %q", err.Error())
	}
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

func TestEnvInjectsCredential(t *testing.T) {
	var stderr bytes.Buffer
	store := &fakeCredentialStore{values: map[string]string{
		"mysql:local": "mysql-secret",
		"redis:cache": "redis-secret",
	}}
	s := NewService(profile.NewService(&memoryProfileStore{}), auth.NewService(store), strings.NewReader(""), &bytes.Buffer{}, &stderr)

	mysqlEnv := strings.Join(s.Env("mysql", "local"), "\n")
	if !strings.Contains(mysqlEnv, "MYSQL_PWD=mysql-secret") {
		t.Fatalf("mysql env does not contain credential")
	}

	redisEnv := strings.Join(s.Env("redis", "cache"), "\n")
	if !strings.Contains(redisEnv, "REDISCLI_AUTH=redis-secret") {
		t.Fatalf("redis env does not contain credential")
	}

	if strings.Contains(stderr.String(), "secret") {
		t.Fatalf("secret leaked in warning output: %q", stderr.String())
	}
}
