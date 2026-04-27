package app

import (
	"bytes"
	"strings"
	"testing"
)

type fakeCredentialStore struct {
	values map[string]string
	err    error
}

func (s *fakeCredentialStore) Set(kind string, profile string, secret string) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[kind+":"+profile] = secret
	return nil
}

func (s *fakeCredentialStore) Get(kind string, profile string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	value, ok := s.values[kind+":"+profile]
	if !ok {
		return "", errCredentialNotFound
	}
	return value, nil
}

func TestParseProfileArgs(t *testing.T) {
	profile, rest, err := parseProfileArgs([]string{"--profile", "local", "--", "-e", "select 1"})
	if err != nil {
		t.Fatalf("parseProfileArgs returned error: %v", err)
	}
	if profile != "local" {
		t.Fatalf("profile = %q, want local", profile)
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

	code := runWithCredentialStore([]string{"redis", "auth", "--profile", "cache", "secret-value"}, strings.NewReader(""), &stdout, &stderr, store)
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

func TestExternalEnvInjectsCredential(t *testing.T) {
	var stderr bytes.Buffer
	store := &fakeCredentialStore{values: map[string]string{
		"mysql:local": "mysql-secret",
		"redis:cache": "redis-secret",
	}}
	r := runner{stderr: &stderr, credentials: store}

	mysqlEnv := strings.Join(r.externalEnv("mysql", "local"), "\n")
	if !strings.Contains(mysqlEnv, "MYSQL_PWD=mysql-secret") {
		t.Fatalf("mysql env does not contain credential")
	}

	redisEnv := strings.Join(r.externalEnv("redis", "cache"), "\n")
	if !strings.Contains(redisEnv, "REDISCLI_AUTH=redis-secret") {
		t.Fatalf("redis env does not contain credential")
	}

	if strings.Contains(stderr.String(), "secret") {
		t.Fatalf("secret leaked in warning output: %q", stderr.String())
	}
}
