package app

import (
	"bytes"
	"strings"
	"testing"
)

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
