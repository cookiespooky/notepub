package serve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HtmlCache struct {
	root  string
	theme string
}

type cacheRecord struct {
	HTML      string `json:"html"`
	RouteETag string `json:"route_etag"`
	StoredAt  string `json:"stored_at"`
}

func NewHtmlCache(root, theme string) *HtmlCache {
	return &HtmlCache{root: root, theme: theme + "-" + cacheSchemaVersion}
}

func (c *HtmlCache) path(siteID, routePath, routeETag string) string {
	return filepath.Join(c.root, "html", safe(siteID), safe(c.theme), safe(routePath)+"-"+safe(routeETag)+".json")
}

func (c *HtmlCache) Read(siteID, routePath, routeETag string) (string, string, error) {
	if routeETag != "" {
		p := c.path(siteID, routePath, routeETag)
		b, err := os.ReadFile(p)
		if err == nil {
			var rec cacheRecord
			if json.Unmarshal(b, &rec) == nil {
				return rec.HTML, "hit", nil
			}
		}
	}

	dir := filepath.Join(c.root, "html", safe(siteID), safe(c.theme))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}
	prefix := safe(routePath) + "-"
	var (
		latestFile string
		latestTime time.Time
	)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = filepath.Join(dir, e.Name())
		}
	}
	if latestFile == "" {
		return "", "", os.ErrNotExist
	}
	b, err := os.ReadFile(latestFile)
	if err != nil {
		return "", "", err
	}
	var rec cacheRecord
	if json.Unmarshal(b, &rec) != nil {
		return "", "", os.ErrInvalid
	}
	return rec.HTML, "stale", nil
}

func (c *HtmlCache) Write(siteID, routePath, routeETag, html string) error {
	rec := cacheRecord{
		HTML:      html,
		RouteETag: routeETag,
		StoredAt:  time.Now().UTC().Format(time.RFC3339),
	}
	p := c.path(siteID, routePath, routeETag)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(rec)
	return os.WriteFile(p, b, 0o644)
}

func safe(val string) string {
	return fmt.Sprintf("%x", []byte(val))
}

const cacheSchemaVersion = "v2"
