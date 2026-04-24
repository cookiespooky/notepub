package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cookiespooky/notepub/internal/indexer"
)

func TestErrorPrecedenceRulesMissing(t *testing.T) {
	withTempDir(t, func() {
		t.Setenv("CONFIG_PATH", "")
		err := buildCmd([]string{"--rules", "nope.yaml"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err.Error() != "rules file not found: nope.yaml" {
			t.Fatalf("unexpected error: %q", err.Error())
		}
		if code := codeFromErr(err); code != 1 {
			t.Fatalf("expected code 1, got %d", code)
		}
	})
}

func TestErrorPrecedenceConfigMissing(t *testing.T) {
	withTempDir(t, func() {
		t.Setenv("CONFIG_PATH", "")
		err := buildCmd([]string{})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err.Error() != "config file not found: config.yaml" {
			t.Fatalf("unexpected error: %q", err.Error())
		}
		if code := codeFromErr(err); code != 1 {
			t.Fatalf("expected code 1, got %d", code)
		}
	})
}

func TestRulesInvalidSchema(t *testing.T) {
	withTempDir(t, func() {
		cfg := `site:
  base_url: "https://example.com"
s3:
  bucket: "bucket"
  access_key: "ak"
  secret_key: "sk"
`
		rules := `version: 1
collections:
  bad:
    kind: "filter"
    materialize: true
    limit: 0
validation:
  materialize_requires_limit: true
`
		if err := os.WriteFile("config.yaml", []byte(cfg), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := os.WriteFile("rules.yaml", []byte(rules), 0o644); err != nil {
			t.Fatalf("write rules: %v", err)
		}

		err := validateCmd([]string{"--config", "config.yaml", "--rules", "rules.yaml"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "rules validation") {
			t.Fatalf("unexpected error: %q", err.Error())
		}
		if code := codeFromErr(err); code != 1 {
			t.Fatalf("expected code 1, got %d", code)
		}
	})
}

func withTempDir(t *testing.T, fn func()) {
	t.Helper()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	fn()
}

func TestNormalizeMarkdownFormat(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "text"},
		{"text", "text"},
		{"TEXT", "text"},
		{"json", "json"},
		{" JSON ", "json"},
		{"xml", ""},
	}
	for _, tc := range cases {
		got := normalizeMarkdownFormat(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeMarkdownFormat(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderMarkdownDiagnosticsJSON(t *testing.T) {
	diags := []indexer.MarkdownDiagnostic{
		{Code: "NP", Severity: "warn", File: "a.md", Line: 2, Message: "m"},
	}
	caps := indexer.MarkdownCapabilities{
		Supported: map[string]bool{"obsidian.callouts": true},
		Used:      map[string]bool{"obsidian.callouts": true},
	}
	b, err := renderMarkdownDiagnostics(diags, caps, "json")
	if err != nil {
		t.Fatalf("renderMarkdownDiagnostics json: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if _, ok := decoded["capabilities"]; !ok {
		t.Fatalf("expected capabilities in json output")
	}
}

func TestWriteMarkdownDiagnosticsFile(t *testing.T) {
	withTempDir(t, func() {
		p := filepath.Join(".", "diag.txt")
		if err := writeMarkdownDiagnostics([]byte("ok\n"), p); err != nil {
			t.Fatalf("writeMarkdownDiagnostics: %v", err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(data) != "ok\n" {
			t.Fatalf("got %q", string(data))
		}
	})
}

func TestRenderMarkdownDiagnosticsText(t *testing.T) {
	diags := []indexer.MarkdownDiagnostic{
		{Code: "NP-X", Severity: "error", File: "a.md", Line: 3, Message: "boom"},
	}
	caps := indexer.MarkdownCapabilities{
		Supported:       map[string]bool{"obsidian.block_refs": false},
		Used:            map[string]bool{"obsidian.block_refs": true},
		UnsupportedUsed: []string{"obsidian.block_refs"},
	}
	b, err := renderMarkdownDiagnostics(diags, caps, "text")
	if err != nil {
		t.Fatalf("render text: %v", err)
	}
	if !strings.Contains(string(b), "NP-X") || !strings.Contains(string(b), "a.md:3") {
		t.Fatalf("unexpected text output: %q", string(b))
	}
	if !strings.Contains(string(b), "Capabilities:") || !strings.Contains(string(b), "Unsupported used:") {
		t.Fatalf("capabilities section is missing: %q", string(b))
	}
}
