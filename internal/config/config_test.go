package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadRuntimeDevOverridesURLs(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
  media_base_url: "https://cdn.example.com/media/"
runtime:
  mode: "dev"
  dev:
    base_url: "http://127.0.0.1:9090/"
    media_base_url: "http://127.0.0.1:9090/media/"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime.Mode != "dev" {
		t.Fatalf("mode = %q, want dev", cfg.Runtime.Mode)
	}
	if cfg.Site.BaseURL != "http://127.0.0.1:9090" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
	if cfg.Site.MediaBaseURL != "http://127.0.0.1:9090/media" {
		t.Fatalf("media_base_url = %q", cfg.Site.MediaBaseURL)
	}
}

func TestLoadRuntimeProdUsesSiteDefaults(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://example.com/root/"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime.Mode != "prod" {
		t.Fatalf("mode = %q, want prod", cfg.Runtime.Mode)
	}
	if cfg.Site.BaseURL != "https://example.com/root" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
	if cfg.Site.MediaBaseURL != "https://example.com/root/media" {
		t.Fatalf("media_base_url = %q", cfg.Site.MediaBaseURL)
	}
}

func TestLoadRuntimeDevInfersBaseURLFromListen(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
runtime:
  mode: "dev"
server:
  listen: ":7777"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.BaseURL != "http://127.0.0.1:7777" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
	if cfg.Site.MediaBaseURL != "http://127.0.0.1:7777/media" {
		t.Fatalf("media_base_url = %q", cfg.Site.MediaBaseURL)
	}
}

func TestLoadRuntimeDevInfersIPv6BaseURLFromListen(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
runtime:
  mode: "dev"
server:
  listen: "[::1]:8080"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.BaseURL != "http://[::1]:8080" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
	if cfg.Site.MediaBaseURL != "http://[::1]:8080/media" {
		t.Fatalf("media_base_url = %q", cfg.Site.MediaBaseURL)
	}
}

func TestLoadRejectsUnknownRuntimeMode(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
runtime:
  mode: "prd"
content:
  source: "local"
  local_dir: "./content"
`)
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatalf("Load should reject unknown runtime.mode")
	}
	if !strings.Contains(err.Error(), "runtime.mode") {
		t.Fatalf("error = %q, want runtime.mode context", err.Error())
	}
}

func TestLoadRuntimeAutoLocalListenChoosesDev(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
runtime:
  mode: "auto"
server:
  listen: "127.0.0.1:4567"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime.Mode != "dev" {
		t.Fatalf("mode = %q, want dev", cfg.Runtime.Mode)
	}
	if cfg.Site.BaseURL != "http://127.0.0.1:4567" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
}

func TestLoadRuntimeAutoCIChoosesProd(t *testing.T) {
	t.Setenv("CI", "true")
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
runtime:
  mode: "auto"
server:
  listen: "127.0.0.1:4567"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Runtime.Mode != "prod" {
		t.Fatalf("mode = %q, want prod", cfg.Runtime.Mode)
	}
	if cfg.Site.BaseURL != "https://prod.example.com" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
}

func TestLoadRuntimeEnvOverridesURLs(t *testing.T) {
	t.Setenv("NOTEPUB_BASE_URL", "https://override.example.com/docs/")
	t.Setenv("NOTEPUB_MEDIA_BASE_URL", "https://override.example.com/docs/media/")
	cfgPath := writeTempConfig(t, `site:
  base_url: "https://prod.example.com/"
runtime:
  mode: "prod"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.BaseURL != "https://override.example.com/docs" {
		t.Fatalf("base_url = %q", cfg.Site.BaseURL)
	}
	if cfg.Site.MediaBaseURL != "https://override.example.com/docs/media" {
		t.Fatalf("media_base_url = %q", cfg.Site.MediaBaseURL)
	}
}
