package serve

import (
	"testing"

	"github.com/cookiespooky/notepub/internal/rules"
)

func TestBuildSettingsMediaAllowlist(t *testing.T) {
	settings := map[string]string{
		"brand_logo":            "/media/notepub.svg",
		"site_icon":             "media/notepub.png",
		"site_default_og_image": "https://example.com/docs/media/notepub.jpg?cache=1",
		"theme_font_url":        "https://fonts.googleapis.com/css2?family=Inter",
	}
	allow := buildSettingsMediaAllowlist(settings)
	if allow == nil {
		t.Fatalf("buildSettingsMediaAllowlist returned nil")
	}
	for _, key := range []string{"notepub.svg", "notepub.png", "notepub.jpg"} {
		if _, ok := allow[key]; !ok {
			t.Fatalf("expected media key %q in allowlist", key)
		}
	}
	if _, ok := allow["css2?family=Inter"]; ok {
		t.Fatalf("unexpected non-media URL in allowlist")
	}
}

func TestMediaAllowedIncludesSettings(t *testing.T) {
	store := NewResolveStore("", rules.Rules{}, false, map[string]string{
		"brand_logo": "/media/logo.svg",
	})
	if !store.MediaAllowed("logo.svg") {
		t.Fatalf("expected settings media key to be allowed")
	}
}
