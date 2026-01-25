package main

import (
	"os"
	"strings"
	"testing"
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
