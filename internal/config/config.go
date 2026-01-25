package config

import (
	"fmt"
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
)

type Config struct {
	Site      SiteConfig   `yaml:"site"`
	S3        S3Config     `yaml:"s3"`
	Paths     PathsConfig  `yaml:"paths"`
	Theme     ThemeConfig  `yaml:"theme"`
	Robots    RobotsConfig `yaml:"robots"`
	Cache     CacheConfig  `yaml:"cache"`
	Server    ServerConfig `yaml:"server"`
	Media     MediaConfig  `yaml:"media"`
	RulesPath string       `yaml:"rules_path"`
}

type SiteConfig struct {
	ID           string   `yaml:"id"`
	BaseURL      string   `yaml:"base_url"`
	MediaBaseURL string   `yaml:"media_base_url"`
	Host         string   `yaml:"host"`
	HostAliases  []string `yaml:"host_aliases"`
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
	cfg.Site.BaseURL = normalizeBaseURL(cfg.Site.BaseURL)
	cfg.Site.MediaBaseURL = normalizeBaseURL(cfg.Site.MediaBaseURL)
	cfg.S3.Prefix = normalizePrefix(cfg.S3.Prefix)
	if cfg.Site.BaseURL == "" {
		return Config{}, fmt.Errorf("site.base_url is required")
	}
	if cfg.S3.Bucket == "" {
		return Config{}, fmt.Errorf("s3.bucket is required")
	}
	if (cfg.S3.AccessKey == "" && cfg.S3.SecretKey != "") || (cfg.S3.AccessKey != "" && cfg.S3.SecretKey == "") {
		return Config{}, fmt.Errorf("s3.access_key and s3.secret_key must be set together")
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) {
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
