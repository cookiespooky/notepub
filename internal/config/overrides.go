package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func applyNoteOverrides(cfg *Config, configDir string) error {
	if strings.TrimSpace(cfg.Overrides.SiteNote) != "" {
		props, err := readNoteFrontmatter(configDir, cfg.Overrides.SiteNote)
		if err != nil {
			if cfg.Overrides.Strict {
				return fmt.Errorf("site overrides: %w", err)
			}
			warnOverride("site_note", err)
		} else {
			applySiteNote(cfg, props)
		}
	}
	if strings.TrimSpace(cfg.Overrides.InterfaceNote) != "" {
		props, err := readNoteFrontmatter(configDir, cfg.Overrides.InterfaceNote)
		if err != nil {
			if cfg.Overrides.Strict {
				return fmt.Errorf("interface overrides: %w", err)
			}
			warnOverride("interface_note", err)
		} else {
			applyInterfaceNote(cfg, props)
		}
	}
	return nil
}

func warnOverride(key string, err error) {
	fmt.Fprintf(os.Stderr, "notepub config warning: overrides.%s ignored: %v\n", key, err)
}

func readNoteFrontmatter(configDir, notePath string) (map[string]string, error) {
	path := strings.TrimSpace(notePath)
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	fm, ok := extractFrontmatter(string(data))
	if !ok {
		return map[string]string{}, nil
	}
	raw := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(fm), &raw); err != nil {
		return nil, fmt.Errorf("parse %s frontmatter: %w", path, err)
	}
	out := map[string]string{}
	for k, v := range raw {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		val := scalarToString(v)
		if strings.TrimSpace(val) == "" {
			continue
		}
		out[key] = strings.TrimSpace(val)
	}
	return out, nil
}

func extractFrontmatter(text string) (string, bool) {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	rest := strings.TrimPrefix(text, "---\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", false
	}
	return rest[:idx], true
}

func scalarToString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

func applySiteNote(cfg *Config, props map[string]string) {
	ensureSettings(cfg)
	mergeSettings(cfg, props)
	if v := strings.TrimSpace(props["site_title"]); v != "" {
		cfg.Site.Title = v
	}
	if v := strings.TrimSpace(props["site_description"]); v != "" {
		cfg.Site.Description = v
	}
	if v := strings.TrimSpace(props["site_url"]); v != "" {
		cfg.Site.BaseURL = v
	}
	if v := strings.TrimSpace(props["site_default_og_image"]); v != "" {
		cfg.Site.DefaultOGImage = v
	}
}

func applyInterfaceNote(cfg *Config, props map[string]string) {
	ensureSettings(cfg)
	mergeSettings(cfg, props)
}

func applySettingsOverrides(cfg *Config) {
	ensureSettings(cfg)
	if v := strings.TrimSpace(cfg.Settings["site_title"]); v != "" {
		cfg.Site.Title = v
	}
	if v := strings.TrimSpace(cfg.Settings["site_description"]); v != "" {
		cfg.Site.Description = v
	}
	if v := strings.TrimSpace(cfg.Settings["site_url"]); v != "" {
		cfg.Site.BaseURL = v
	}
	if v := strings.TrimSpace(cfg.Settings["site_default_og_image"]); v != "" {
		cfg.Site.DefaultOGImage = v
	}
}

func mergeSettings(cfg *Config, props map[string]string) {
	ensureSettings(cfg)
	for key, val := range props {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(val) == "" {
			continue
		}
		cfg.Settings[key] = val
	}
}

func finalizeSettings(cfg *Config) {
	ensureSettings(cfg)
	setDefault(cfg.Settings, "site_title", cfg.Site.Title)
	setDefault(cfg.Settings, "site_description", cfg.Site.Description)
	cfg.Settings["site_url"] = cfg.Site.BaseURL
	setDefault(cfg.Settings, "site_language", "en")
	setDefault(cfg.Settings, "site_default_og_image", cfg.Site.DefaultOGImage)
	cfg.Settings["site_default_og_image"] = resolveAssetOrMediaURL(cfg.Settings["site_default_og_image"], cfg.Site.BaseURL, cfg.Site.MediaBaseURL)
}

func ensureSettings(cfg *Config) {
	if cfg.Settings == nil {
		cfg.Settings = map[string]string{}
	}
}

func setDefault(m map[string]string, key, val string) {
	if strings.TrimSpace(m[key]) == "" {
		m[key] = val
	}
}

func resolveAssetOrMediaURL(v, baseURL, mediaBaseURL string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if _, err := url.ParseRequestURI(v); err == nil && (strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")) {
		return v
	}
	if strings.HasPrefix(v, "/media/") {
		return strings.TrimRight(mediaBaseURL, "/") + strings.TrimPrefix(v, "/media")
	}
	if strings.HasPrefix(v, "/assets/") {
		return strings.TrimRight(baseURL, "/") + v
	}
	return v
}
