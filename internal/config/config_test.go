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
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
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
	t.Setenv("GITHUB_ACTIONS", "true")
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

func TestLoadAppliesFlatNoteOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "Site.md"), []byte(`---
site_title: Docs From Note
site_description: Description From Note
site_language: ru
site_default_og_image: /media/cover.png
brand_name: Brand From Note
brand_logo: /media/logo.svg
theme_accent: "#2563eb"
theme_link: "#0f766e"
theme_font: serif
theme_heading_font: inter
theme_radius: 8
custom_template_label: Custom value
---
`), 0o644); err != nil {
		t.Fatalf("write Site.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Interface.md"), []byte(`---
hero_title: Заголовок героя
cta_primary_label: Читать дальше
empty_state_text: Пока ничего нет
---
`), 0o644); err != nil {
		t.Fatalf("write Interface.md: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(`site:
  base_url: "http://127.0.0.1:8080/"
  title: "Fallback"
  description: "Fallback description"
overrides:
  site_note: "./Site.md"
  interface_note: "./Interface.md"
content:
  source: "local"
  local_dir: "./content"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.Title != "Docs From Note" {
		t.Fatalf("site title = %q", cfg.Site.Title)
	}
	if cfg.Site.Description != "Description From Note" {
		t.Fatalf("site description = %q", cfg.Site.Description)
	}
	assertSetting := func(key, want string) {
		t.Helper()
		if got := cfg.Settings[key]; got != want {
			t.Fatalf("settings[%q] = %q, want %q", key, got, want)
		}
	}
	assertSetting("site_title", "Docs From Note")
	assertSetting("site_description", "Description From Note")
	assertSetting("site_url", "http://127.0.0.1:8080")
	assertSetting("site_language", "ru")
	assertSetting("site_default_og_image", "http://127.0.0.1:8080/media/cover.png")
	assertSetting("brand_name", "Brand From Note")
	assertSetting("brand_logo", "/media/logo.svg")
	assertSetting("theme_accent", "#2563eb")
	assertSetting("theme_link", "#0f766e")
	assertSetting("theme_radius", "8")
	assertSetting("custom_template_label", "Custom value")
	assertSetting("hero_title", "Заголовок героя")
	assertSetting("cta_primary_label", "Читать дальше")
	assertSetting("empty_state_text", "Пока ничего нет")
}

func TestLoadUsesConfigSettingsAsFallbackWithoutNotes(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "http://127.0.0.1:8080/"
  title: "Fallback"
  description: "Fallback description"
settings:
  site_title: "Title From Settings"
  site_description: "Description From Settings"
  site_language: "ru"
  site_default_og_image: "/media/from-settings.png"
  custom_template_label: "Custom From Settings"
content:
  source: "local"
  local_dir: "./content"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.Title != "Title From Settings" {
		t.Fatalf("site title = %q", cfg.Site.Title)
	}
	if cfg.Site.Description != "Description From Settings" {
		t.Fatalf("site description = %q", cfg.Site.Description)
	}
	if got := cfg.Settings["site_default_og_image"]; got != "http://127.0.0.1:8080/media/from-settings.png" {
		t.Fatalf("site_default_og_image = %q", got)
	}
	if got := cfg.Settings["custom_template_label"]; got != "Custom From Settings" {
		t.Fatalf("custom_template_label = %q", got)
	}
}

func TestLoadMissingNoteOverrideFallsBackWhenNotStrict(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "http://127.0.0.1:8080/"
  title: "Fallback"
settings:
  site_title: "Title From Settings"
overrides:
  site_note: "./Site.md"
content:
  source: "local"
  local_dir: "./content"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.Title != "Title From Settings" {
		t.Fatalf("site title = %q", cfg.Site.Title)
	}
}

func TestLoadMissingNoteOverrideFailsWhenStrict(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "http://127.0.0.1:8080/"
overrides:
  site_note: "./Site.md"
  strict: true
content:
  source: "local"
  local_dir: "./content"
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatalf("Load should fail in strict mode")
	}
	if !strings.Contains(err.Error(), "site overrides") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadRejectsUnknownCompatMode(t *testing.T) {
	cfgPath := writeTempConfig(t, `compat_mode: "nextgen"
site:
  base_url: "http://127.0.0.1:8080/"
content:
  source: "local"
  local_dir: "./content"
`)
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatalf("Load should reject unknown compat_mode")
	}
	if !strings.Contains(err.Error(), "compat_mode") {
		t.Fatalf("error = %q, want compat_mode context", err.Error())
	}
}

func TestLoadAutoCompatResolvesLegacyWithoutSettingsAndOverrides(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "http://127.0.0.1:8080/"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CompatMode != "legacy" {
		t.Fatalf("compat_mode = %q, want legacy", cfg.CompatMode)
	}
}

func TestLoadAutoCompatResolvesModernWhenSettingsPresent(t *testing.T) {
	cfgPath := writeTempConfig(t, `site:
  base_url: "http://127.0.0.1:8080/"
settings:
  site_title: "From settings"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CompatMode != "modern" {
		t.Fatalf("compat_mode = %q, want modern", cfg.CompatMode)
	}
}

func TestLoadCompatModeCanBeOverriddenByEnv(t *testing.T) {
	t.Setenv("NOTEPUB_COMPAT_MODE", "legacy")
	cfgPath := writeTempConfig(t, `compat_mode: "modern"
site:
  base_url: "http://127.0.0.1:8080/"
content:
  source: "local"
  local_dir: "./content"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CompatMode != "legacy" {
		t.Fatalf("compat_mode = %q, want legacy", cfg.CompatMode)
	}
}

func TestLoadLegacyCompatSkipsNoteOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "Site.md"), []byte(`---
site_title: From Note
---
`), 0o644); err != nil {
		t.Fatalf("write Site.md: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(`compat_mode: "legacy"
site:
  base_url: "http://127.0.0.1:8080/"
  title: "Fallback"
settings:
  site_title: "From Settings"
overrides:
  site_note: "./Site.md"
content:
  source: "local"
  local_dir: "./content"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CompatMode != "legacy" {
		t.Fatalf("compat_mode = %q, want legacy", cfg.CompatMode)
	}
	if cfg.Site.Title != "From Settings" {
		t.Fatalf("site title = %q, want From Settings", cfg.Site.Title)
	}
}

func TestLoadModernCompatAppliesNoteOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "Site.md"), []byte(`---
site_title: From Note
---
`), 0o644); err != nil {
		t.Fatalf("write Site.md: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(`compat_mode: "modern"
site:
  base_url: "http://127.0.0.1:8080/"
  title: "Fallback"
settings:
  site_title: "From Settings"
overrides:
  site_note: "./Site.md"
content:
  source: "local"
  local_dir: "./content"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CompatMode != "modern" {
		t.Fatalf("compat_mode = %q, want modern", cfg.CompatMode)
	}
	if cfg.Site.Title != "From Note" {
		t.Fatalf("site title = %q, want From Note", cfg.Site.Title)
	}
}
