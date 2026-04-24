package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "config.yaml"
	defaultFileRoot   = "/var/lib/notepub"
	defaultCacheRoot  = "/var/cache/notepub"
	defaultThemesDir  = "/opt/notepub/themes"
	defaultThemeName  = "seo-minimal"
	defaultLocalDir   = "markdown"
)

type Config struct {
	Site         SiteConfig        `yaml:"site"`
	Runtime      RuntimeConfig     `yaml:"runtime"`
	S3           S3Config          `yaml:"s3"`
	Content      ContentConfig     `yaml:"content"`
	Markdown     MarkdownConfig    `yaml:"markdown"`
	OGTypeByType map[string]string `yaml:"og_type_by_type"`
	Paths        PathsConfig       `yaml:"paths"`
	Theme        ThemeConfig       `yaml:"theme"`
	Robots       RobotsConfig      `yaml:"robots"`
	Cache        CacheConfig       `yaml:"cache"`
	Server       ServerConfig      `yaml:"server"`
	Media        MediaConfig       `yaml:"media"`
	RulesPath    string            `yaml:"rules_path"`
}

type SiteConfig struct {
	ID             string   `yaml:"id"`
	BaseURL        string   `yaml:"base_url"`
	Title          string   `yaml:"title"`
	Description    string   `yaml:"description"`
	DefaultOGImage string   `yaml:"default_og_image"`
	MediaBaseURL   string   `yaml:"media_base_url"`
	Host           string   `yaml:"host"`
	HostAliases    []string `yaml:"host_aliases"`
}

type RuntimeConfig struct {
	Mode string      `yaml:"mode"`
	Dev  RuntimeURLs `yaml:"dev"`
	Prod RuntimeURLs `yaml:"prod"`
}

type RuntimeURLs struct {
	BaseURL      string `yaml:"base_url"`
	MediaBaseURL string `yaml:"media_base_url"`
}

type S3Config struct {
	Endpoint       string `yaml:"endpoint"`
	Region         string `yaml:"region"`
	ForcePathStyle bool   `yaml:"force_path_style"`
	Bucket         string `yaml:"bucket"`
	Prefix         string `yaml:"prefix"`
	AccessKey      string `yaml:"access_key"`
	SecretKey      string `yaml:"secret_key"`
	Anonymous      bool   `yaml:"anonymous"`
}

type ContentConfig struct {
	Source   string `yaml:"source"`
	LocalDir string `yaml:"local_dir"`
}

type MarkdownConfig struct {
	HTMLPolicy string `yaml:"html_policy"`
}

type PathsConfig struct {
	FileRoot     string `yaml:"file_root"`
	ArtifactsDir string `yaml:"artifacts_dir"`
	SnapshotFile string `yaml:"snapshot_file"`
	CacheRoot    string `yaml:"cache_root"`
}

type ThemeConfig struct {
	Dir             string `yaml:"dir"`
	Name            string `yaml:"name"`
	TemplatesSubdir string `yaml:"templates_subdir"`
	AssetsSubdir    string `yaml:"assets_subdir"`
}

type RobotsConfig struct {
	Extra    string   `yaml:"extra"`
	Disallow []string `yaml:"disallow"`
}

type CacheConfig struct {
	HTMLTTLSeconds      int `yaml:"html_ttl_seconds"`
	StaleIfErrorSeconds int `yaml:"stale_if_error_seconds"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type MediaConfig struct {
	ExposeAllUnderPrefix bool `yaml:"expose_all_under_prefix"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = defaultConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.RulesPath == "" {
		cfg.RulesPath = filepath.Join(filepath.Dir(path), "rules.yaml")
	}
	applyDefaults(&cfg)
	if cfg.Site.ID == "" {
		cfg.Site.ID = "default"
	}
	mode, err := normalizeRuntimeMode(cfg.Runtime.Mode)
	if err != nil {
		return Config{}, err
	}
	cfg.Runtime.Mode = mode
	if err := ApplyRuntimeURLs(&cfg); err != nil {
		return Config{}, err
	}
	cfg.Site.BaseURL = normalizeBaseURL(cfg.Site.BaseURL)
	cfg.Site.MediaBaseURL = normalizeBaseURL(cfg.Site.MediaBaseURL)
	cfg.S3.Prefix = normalizePrefix(cfg.S3.Prefix)
	cfg.Content.Source = strings.ToLower(strings.TrimSpace(cfg.Content.Source))
	if cfg.Content.Source == "" {
		if cfg.S3.Bucket == "" {
			cfg.Content.Source = "local"
		} else {
			cfg.Content.Source = "s3"
		}
	}
	if cfg.Content.Source == "local" {
		if cfg.Content.LocalDir == "" {
			cfg.Content.LocalDir = defaultLocalDir
		}
		if !filepath.IsAbs(cfg.Content.LocalDir) {
			cfg.Content.LocalDir = filepath.Join(filepath.Dir(path), cfg.Content.LocalDir)
		}
		cfg.Content.LocalDir = filepath.Clean(cfg.Content.LocalDir)
	}
	if cfg.Site.BaseURL == "" {
		return Config{}, fmt.Errorf("site.base_url is required")
	}
	switch cfg.Content.Source {
	case "s3":
		if cfg.S3.Bucket == "" {
			return Config{}, fmt.Errorf("s3.bucket is required")
		}
	case "local":
		if cfg.Content.LocalDir == "" {
			return Config{}, fmt.Errorf("content.local_dir is required for local source")
		}
	default:
		return Config{}, fmt.Errorf("content.source must be \"s3\" or \"local\"")
	}
	if cfg.Content.Source == "s3" {
		if (cfg.S3.AccessKey == "" && cfg.S3.SecretKey != "") || (cfg.S3.AccessKey != "" && cfg.S3.SecretKey == "") {
			return Config{}, fmt.Errorf("s3.access_key and s3.secret_key must be set together")
		}
	}
	cfg.Markdown.HTMLPolicy = normalizeHTMLPolicy(cfg.Markdown.HTMLPolicy)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Content.Source == "" {
		if cfg.S3.Bucket == "" {
			cfg.Content.Source = "local"
		} else {
			cfg.Content.Source = "s3"
		}
	}
	if cfg.Content.LocalDir == "" {
		cfg.Content.LocalDir = defaultLocalDir
	}
	if cfg.Paths.FileRoot == "" {
		cfg.Paths.FileRoot = defaultFileRoot
	}
	if cfg.Paths.ArtifactsDir == "" {
		cfg.Paths.ArtifactsDir = filepath.Join(cfg.Paths.FileRoot, "artifacts")
	}
	if cfg.Paths.SnapshotFile == "" {
		cfg.Paths.SnapshotFile = filepath.Join(cfg.Paths.FileRoot, "snapshot", "objects.json")
	}
	if cfg.Paths.CacheRoot == "" {
		cfg.Paths.CacheRoot = defaultCacheRoot
	}
	if cfg.Theme.Dir == "" {
		cfg.Theme.Dir = defaultThemesDir
	}
	if cfg.Theme.Name == "" {
		cfg.Theme.Name = defaultThemeName
	}
	if cfg.Theme.TemplatesSubdir == "" {
		cfg.Theme.TemplatesSubdir = "templates"
	}
	if cfg.Theme.AssetsSubdir == "" {
		cfg.Theme.AssetsSubdir = "assets"
	}
	if cfg.Cache.HTMLTTLSeconds == 0 {
		cfg.Cache.HTMLTTLSeconds = 600
	}
	if cfg.Cache.StaleIfErrorSeconds == 0 {
		cfg.Cache.StaleIfErrorSeconds = 604800
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8081"
	}
	if cfg.S3.Region == "" {
		cfg.S3.Region = "us-east-1"
	}
	if cfg.Markdown.HTMLPolicy == "" {
		cfg.Markdown.HTMLPolicy = "safe"
	}
	if cfg.Runtime.Mode == "" {
		cfg.Runtime.Mode = "prod"
	}
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	return baseURL
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix == "" {
		return ""
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func normalizeHTMLPolicy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "safe":
		return "safe"
	case "unsafe":
		return "unsafe"
	case "deny":
		return "deny"
	default:
		return "safe"
	}
}

func normalizeRuntimeMode(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "prod", "production":
		return "prod", nil
	case "dev", "development":
		return "dev", nil
	case "auto":
		return "auto", nil
	default:
		return "", fmt.Errorf("runtime.mode must be \"dev\", \"prod\", or \"auto\"")
	}
}

func ApplyRuntimeURLs(cfg *Config) error {
	mode, err := resolveRuntimeMode(cfg)
	if err != nil {
		return err
	}
	base := ""
	media := ""
	if mode == "dev" {
		inferred := inferDevBaseURL(cfg.Server.Listen)
		base = firstNonEmptyURL(cfg.Runtime.Dev.BaseURL, inferred, cfg.Site.BaseURL)
		media = firstNonEmptyURL(cfg.Runtime.Dev.MediaBaseURL, cfg.Site.MediaBaseURL)
	} else {
		base = firstNonEmptyURL(cfg.Runtime.Prod.BaseURL, cfg.Site.BaseURL)
		media = firstNonEmptyURL(cfg.Runtime.Prod.MediaBaseURL, cfg.Site.MediaBaseURL)
	}
	if media == "" && base != "" {
		media = strings.TrimRight(base, "/") + "/media/"
	}
	if envBase := strings.TrimSpace(os.Getenv("NOTEPUB_BASE_URL")); envBase != "" {
		base = envBase
	}
	if envMedia := strings.TrimSpace(os.Getenv("NOTEPUB_MEDIA_BASE_URL")); envMedia != "" {
		media = envMedia
	}
	if media == "" && base != "" {
		media = strings.TrimRight(base, "/") + "/media/"
	}
	cfg.Site.BaseURL = normalizeBaseURL(base)
	cfg.Site.MediaBaseURL = normalizeBaseURL(media)
	cfg.Runtime.Mode = mode
	return nil
}

func resolveRuntimeMode(cfg *Config) (string, error) {
	if envMode := strings.TrimSpace(os.Getenv("NOTEPUB_RUNTIME_MODE")); envMode != "" {
		mode, err := normalizeRuntimeMode(envMode)
		if err != nil {
			return "", fmt.Errorf("NOTEPUB_RUNTIME_MODE: %w", err)
		}
		switch mode {
		case "dev", "prod":
			return mode, nil
		case "auto":
			return "", fmt.Errorf("NOTEPUB_RUNTIME_MODE must be \"dev\" or \"prod\"")
		}
	}
	requested, err := normalizeRuntimeMode(cfg.Runtime.Mode)
	if err != nil {
		return "", err
	}
	if requested != "auto" {
		return requested, nil
	}
	if envIsTrue("CI") || envIsTrue("GITHUB_ACTIONS") {
		return "prod", nil
	}
	if isLocalListen(cfg.Server.Listen) || isLocalURL(cfg.Site.BaseURL) {
		return "dev", nil
	}
	return "prod", nil
}

func envIsTrue(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isLocalListen(listen string) bool {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return true
	}
	if strings.HasPrefix(listen, ":") {
		return true
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	host = strings.TrimSpace(host)
	switch host {
	case "", "127.0.0.1", "localhost", "::1", "0.0.0.0", "::":
		return true
	default:
		return false
	}
}

func isLocalURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	switch h {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func firstNonEmptyURL(vals ...string) string {
	for _, v := range vals {
		s := strings.TrimSpace(v)
		if s != "" {
			return s
		}
	}
	return ""
}

func inferDevBaseURL(listen string) string {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return "http://127.0.0.1:8080/"
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		trim := strings.TrimPrefix(listen, ":")
		if trim == "" {
			return "http://127.0.0.1:8080/"
		}
		return "http://127.0.0.1:" + trim + "/"
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}
