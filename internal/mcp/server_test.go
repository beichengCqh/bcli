package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
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

func TestMCPInitializeAndToolsList(t *testing.T) {
	output := runServer(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"dev"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)

	if !strings.Contains(output, `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("initialize response missing protocol version: %s", output)
	}
	if !strings.Contains(output, `"name":"bcli.profile.list"`) || !strings.Contains(output, `"name":"bcli.auth.mysql"`) {
		t.Fatalf("tools/list response missing expected tools: %s", output)
	}
}

func TestMCPProfileSetAndResourceReadDoNotExposePassword(t *testing.T) {
	credentials := &fakeCredentialStore{}
	profiles := &memoryProfileStore{}
	output := runServerWithStores(t, credentials, profiles,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bcli.profile.set","arguments":{"kind":"mysql","name":"local","host":"127.0.0.1","port":3306,"user":"root","database":"app"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"bcli.auth.mysql","arguments":{"profile":"local","password":"secret-value"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"bcli://profiles/mysql/local"}}`,
	)

	if credentials.values["mysql:local"] != "secret-value" {
		t.Fatalf("credential was not stored")
	}
	if strings.Contains(output, "secret-value") || strings.Contains(output, "password") {
		t.Fatalf("MCP output leaked credential-looking content: %s", output)
	}
	if !strings.Contains(output, `\"hasCredential\": true`) {
		t.Fatalf("resource did not include credential status: %s", output)
	}
}

func TestMCPProfileResourceURIEscapesName(t *testing.T) {
	profiles := &memoryProfileStore{cfg: profile.Config{
		MySQL: map[string]profile.ExternalProfile{
			"local/name ?": {Host: "127.0.0.1"},
		},
	}}
	output := runServerWithStores(t, &fakeCredentialStore{}, profiles,
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"bcli://profiles/mysql/local%2Fname%20%3F"}}`,
	)

	if !strings.Contains(output, `bcli://profiles/mysql/local%2Fname%20%3F`) {
		t.Fatalf("resources/list missing escaped URI: %s", output)
	}
	if !strings.Contains(output, `\"name\": \"local/name ?\"`) {
		t.Fatalf("resources/read did not resolve escaped profile name: %s", output)
	}
}

func TestMCPPromptsGet(t *testing.T) {
	output := runServer(t,
		`{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"prompts/get","params":{"name":"bcli.prompt.inspect_profiles"}}`,
	)

	if !strings.Contains(output, `"name":"bcli.prompt.inspect_profiles"`) {
		t.Fatalf("prompts/list missing prompt: %s", output)
	}
	if !strings.Contains(output, "Never ask for or reveal stored credentials") {
		t.Fatalf("prompts/get missing safety instruction: %s", output)
	}
}

func runServer(t *testing.T, messages ...string) string {
	t.Helper()
	return runServerWithStores(t, &fakeCredentialStore{}, &memoryProfileStore{}, messages...)
}

func runServerWithStores(t *testing.T, credentials *fakeCredentialStore, profiles *memoryProfileStore, messages ...string) string {
	t.Helper()
	var stdin bytes.Buffer
	for _, message := range messages {
		stdin.WriteString(message)
		stdin.WriteByte('\n')
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	server := NewServer(&stdin, &stdout, &stderr, auth.NewService(credentials), profile.NewService(profiles))
	if err := server.Serve(); err != nil {
		t.Fatalf("Serve returned error: %v, stderr=%s", err, stderr.String())
	}
	assertJSONLines(t, stdout.String())
	return stdout.String()
}

func assertJSONLines(t *testing.T, output string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		var value map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			t.Fatalf("output line is not valid JSON: %s", scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan output: %v", err)
	}
}
