package serve

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/indexer"
	"github.com/cookiespooky/notepub/internal/localutil"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/s3util"
	"github.com/yuin/goldmark/parser"
)

type BuildOptions struct {
	DistDir        string
	ArtifactsDir   string
	NoIndex        bool
	GenerateSearch bool
}

func Build(ctx context.Context, cfg config.Config, rulesCfg rules.Rules, opts BuildOptions) error {
	rulesDir := filepath.Dir(cfg.RulesPath)
	artifactsDir := opts.ArtifactsDir
	if artifactsDir == "" {
		if rulesDir == "" || rulesDir == "." {
			artifactsDir = "artifacts"
		} else {
			artifactsDir = filepath.Join(rulesDir, "artifacts")
		}
	}
	distDir := opts.DistDir
	if distDir == "" {
		if rulesDir == "" || rulesDir == "." {
			distDir = "dist"
		} else {
			distDir = filepath.Join(rulesDir, "dist")
		}
	}

	resolvePath := filepath.Join(artifactsDir, "resolve.json")
	if _, err := os.Stat(resolvePath); err != nil {
		if opts.NoIndex {
			return fmt.Errorf("resolve.json missing and --no-index set: %w", err)
		}
		cfgForIndex := cfg
		cfgForIndex.Paths.ArtifactsDir = artifactsDir
		if err := indexer.Run(ctx, cfgForIndex); err != nil {
			return fmt.Errorf("index before build: %w", err)
		}
	}

	idx, err := loadResolveIndex(resolvePath)
	if err != nil {
		return err
	}

	themeDir := filepath.Join(cfg.Theme.Dir, cfg.Theme.Name)
	theme, err := LoadTheme(themeDir, cfg.Theme.TemplatesSubdir, cfg.Theme.AssetsSubdir)
	if err != nil {
		return fmt.Errorf("load theme: %w", err)
	}

	var s3client *s3.Client
	if cfg.Content.Source == "s3" {
		s3client, err = s3util.NewClient(ctx, s3util.Config{
			Endpoint:       cfg.S3.Endpoint,
			Region:         cfg.S3.Region,
			ForcePathStyle: cfg.S3.ForcePathStyle,
			Bucket:         cfg.S3.Bucket,
			Prefix:         cfg.S3.Prefix,
			AccessKey:      cfg.S3.AccessKey,
			SecretKey:      cfg.S3.SecretKey,
			Anonymous:      cfg.S3.Anonymous,
		})
		if err != nil {
			return fmt.Errorf("s3 client: %w", err)
		}
	}

	if err := resetDir(distDir); err != nil {
		return err
	}
	if err := copyThemeAssets(theme, distDir); err != nil {
		return err
	}
	if err := copyArtifacts(idx, cfg, rulesCfg, artifactsDir, distDir, opts.GenerateSearch); err != nil {
		return err
	}

	md := newMarkdownRenderer()
	wikiMap := buildWikiMap(idx)
	paths := sortedRoutes(idx.Routes)
	for _, pathVal := range paths {
		route := idx.Routes[pathVal]
		meta, ok := idx.Meta[pathVal]
		if !ok {
			continue
		}
		outPath := outputPath(distDir, pathVal)
		if route.Status == 301 && route.RedirectTo != "" {
			if err := writeRedirectPage(outPath, cfg.Site.BaseURL, route.RedirectTo); err != nil {
				return err
			}
			continue
		}
		if route.Status != 200 || route.S3Key == "" {
			continue
		}

		var body []byte
		switch cfg.Content.Source {
		case "local":
			body, err = localutil.FetchObject(cfg.Content.LocalDir, route.S3Key)
		case "s3":
			if s3client == nil {
				return fmt.Errorf("s3 client is not initialized")
			}
			body, err = s3util.FetchObject(ctx, s3client, cfg.S3.Bucket, route.S3Key)
		default:
			return fmt.Errorf("unsupported content source: %s", cfg.Content.Source)
		}
		if err != nil {
			return fmt.Errorf("fetch %s: %w", route.S3Key, err)
		}
		rendered, err := renderMarkdownForBuild(string(body), route.S3Key, cfg.S3.Prefix, cfg.Site.MediaBaseURL, wikiMap, md)
		if err != nil {
			return fmt.Errorf("render markdown %s: %w", route.S3Key, err)
		}

		meta = normalizeMetaMediaURLs(meta, cfg.Site.MediaBaseURL)
		data := buildPageData(meta, rendered)
		data.Template = templateForType(meta.Type, rulesCfg)
		data.Page.NoIndex = route.NoIndex
		data.SearchMode = "static"
		if pathVal == "/" {
			data.IsHome = true
		}
		data.Collections = buildCollections(idx, rulesCfg, pathVal)
		html, err := theme.RenderPage(data)
		if err != nil {
			return fmt.Errorf("render page %s: %w", pathVal, err)
		}
		if err := writeFile(outPath, []byte(html)); err != nil {
			return err
		}
	}

	notFound, err := theme.RenderNotFound()
	if err != nil {
		notFound = "Not Found"
	}
	if err := writeFile(filepath.Join(distDir, "404.html"), []byte(notFound)); err != nil {
		return err
	}

	return nil
}

func loadResolveIndex(path string) (models.ResolveIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.ResolveIndex{}, fmt.Errorf("read resolve: %w", err)
	}
	var idx models.ResolveIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return models.ResolveIndex{}, fmt.Errorf("parse resolve: %w", err)
	}
	return idx, nil
}

func resetDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func copyThemeAssets(theme *Theme, distDir string) error {
	assetFS := theme.AssetFS()
	assetRoot := theme.assetsSubdir
	if assetRoot == "" {
		assetRoot = "assets"
	}
	if _, err := fs.Stat(assetFS, assetRoot); err != nil {
		return nil
	}
	return fs.WalkDir(assetFS, assetRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, assetRoot)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		data, err := fs.ReadFile(assetFS, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(distDir, "assets", filepath.FromSlash(rel))
		return writeFile(dst, data)
	})
}

func copyArtifacts(idx models.ResolveIndex, cfg config.Config, rulesCfg rules.Rules, artifactsDir, distDir string, generateSearch bool) error {
	if ok, err := copySitemaps(artifactsDir, distDir); err != nil {
		return err
	} else if !ok {
		if err := writeMinimalSitemap(distDir, cfg.Site.BaseURL, idx, rulesCfg); err != nil {
			return err
		}
	}
	if ok, err := copyRobots(artifactsDir, distDir); err != nil {
		return err
	} else if !ok {
		if err := writeMinimalRobots(distDir, cfg.Site.BaseURL); err != nil {
			return err
		}
	}

	searchPath := filepath.Join(artifactsDir, "search.json")
	if exists(searchPath) {
		if err := copyFile(searchPath, filepath.Join(distDir, "search.json")); err != nil {
			return err
		}
	} else if generateSearch {
		if err := writeSearchIndex(distDir, idx, rulesCfg); err != nil {
			return err
		}
	}
	return nil
}

func copySitemaps(artifactsDir, distDir string) (bool, error) {
	indexPath := filepath.Join(artifactsDir, "sitemap-index.xml")
	if exists(indexPath) {
		if err := copyFile(indexPath, filepath.Join(distDir, "sitemap-index.xml")); err != nil {
			return true, err
		}
		if err := copyFile(indexPath, filepath.Join(distDir, "sitemap.xml")); err != nil {
			return true, err
		}
		matches, err := filepath.Glob(filepath.Join(artifactsDir, "sitemap-*.xml"))
		if err != nil {
			return true, err
		}
		for _, p := range matches {
			if filepath.Base(p) == "sitemap-index.xml" {
				continue
			}
			if err := copyFile(p, filepath.Join(distDir, filepath.Base(p))); err != nil {
				return true, err
			}
		}
		return true, nil
	}
	sitemapPath := filepath.Join(artifactsDir, "sitemap.xml")
	if exists(sitemapPath) {
		return true, copyFile(sitemapPath, filepath.Join(distDir, "sitemap.xml"))
	}
	return false, nil
}

func copyRobots(artifactsDir, distDir string) (bool, error) {
	path := filepath.Join(artifactsDir, "robots.txt")
	if !exists(path) {
		return false, nil
	}
	return true, copyFile(path, filepath.Join(distDir, "robots.txt"))
}

func writeMinimalRobots(distDir, baseURL string) error {
	body := strings.Join([]string{
		"User-agent: *",
		"Allow: /",
		"Sitemap: " + buildAbsoluteURL(baseURL, "/sitemap.xml"),
	}, "\n") + "\n"
	return writeFile(filepath.Join(distDir, "robots.txt"), []byte(body))
}

func writeMinimalSitemap(distDir, baseURL string, idx models.ResolveIndex, cfg rules.Rules) error {
	type urlEntry struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod,omitempty"`
	}
	urls := []urlEntry{}
	for p, rt := range idx.Routes {
		if rt.Status != 200 || rt.NoIndex {
			continue
		}
		meta, ok := idx.Meta[p]
		if !ok {
			continue
		}
		if len(cfg.Sitemap.IncludeTypes) > 0 && !typeAllowed(meta.Type, cfg.Sitemap.IncludeTypes) {
			continue
		}
		if cfg.Sitemap.ExcludeDrafts && boolFromMeta(meta.FM, "draft") {
			continue
		}
		loc := buildAbsoluteURL(baseURL, p)
		lastmod := ""
		if rt.LastModified != "" {
			if t, err := time.Parse(time.RFC3339, rt.LastModified); err == nil {
				lastmod = t.UTC().Format("2006-01-02")
			}
		}
		urls = append(urls, urlEntry{Loc: loc, LastMod: lastmod})
	}
	urlset := struct {
		XMLName xml.Name   `xml:"urlset"`
		Xmlns   string     `xml:"xmlns,attr"`
		URLs    []urlEntry `xml:"url"`
	}{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}
	buf, err := xml.Marshal(urlset)
	if err != nil {
		return err
	}
	xmlBody := []byte(xml.Header + string(buf))
	return writeFile(filepath.Join(distDir, "sitemap.xml"), xmlBody)
}

func writeSearchIndex(distDir string, idx models.ResolveIndex, rulesCfg rules.Rules) error {
	docs := buildSearchIndex(idx, rulesCfg)
	items := make([]SearchItem, 0, len(docs))
	for _, doc := range docs {
		items = append(items, doc.toItem())
	}
	payload := struct {
		GeneratedAt string       `json:"generated_at"`
		Items       []SearchItem `json:"items"`
	}{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Items:       items,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(distDir, "search.json"), b)
}

func renderMarkdownForBuild(markdown, baseKey, prefix, mediaBase string, wikiMap map[string]string, mdRenderer markdownRenderer) (string, error) {
	markdown = normalizeMarkdownImagesForBuild(markdown, baseKey, prefix, mediaBase)
	markdown = normalizeMarkdownLinks(markdown, wikiMap)
	var buf strings.Builder
	if err := mdRenderer.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func normalizeMetaMediaURLs(meta models.MetaEntry, mediaBase string) models.MetaEntry {
	if mediaBase == "" {
		return meta
	}
	base := strings.TrimRight(mediaBase, "/")
	update := func(v string) string {
		if v == "" || isExternal(v) {
			return v
		}
		if strings.HasPrefix(v, "/media/") {
			key := strings.TrimPrefix(v, "/media/")
			key = strings.TrimPrefix(key, "/")
			if key == "" {
				return v
			}
			return base + "/" + escapePath(key)
		}
		if strings.HasPrefix(v, "/") {
			return v
		}
		return base + "/" + escapePath(strings.TrimPrefix(v, "/"))
	}
	meta.Image = update(meta.Image)
	if meta.OpenGraph != nil {
		clone := map[string]string{}
		for k, v := range meta.OpenGraph {
			if strings.EqualFold(k, "image") {
				clone[k] = update(v)
			} else {
				clone[k] = v
			}
		}
		meta.OpenGraph = clone
	}
	return meta
}

func outputPath(distDir, routePath string) string {
	clean := path.Clean("/" + strings.TrimSpace(routePath))
	if clean == "/" {
		return filepath.Join(distDir, "index.html")
	}
	clean = strings.TrimPrefix(clean, "/")
	return filepath.Join(distDir, filepath.FromSlash(clean), "index.html")
}

func writeRedirectPage(outPath, baseURL, target string) error {
	abs := target
	if !isExternal(target) {
		abs = buildAbsoluteURL(baseURL, target)
	}
	html := `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta http-equiv="refresh" content="0; url=` + htmlEscape(abs) + `">
    <link rel="canonical" href="` + htmlEscape(abs) + `">
    <title>Redirecting…</title>
  </head>
  <body>
    <p>Redirecting to <a href="` + htmlEscape(abs) + `">` + htmlEscape(abs) + `</a></p>
  </body>
</html>`
	return writeFile(outPath, []byte(html))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sortedRoutes(routes map[string]models.RouteEntry) []string {
	out := make([]string, 0, len(routes))
	for p := range routes {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func templateForType(typeName string, cfg rules.Rules) string {
	if typeName == "" {
		return ""
	}
	td, ok := cfg.Types[typeName]
	if !ok {
		return ""
	}
	return td.Template
}

type markdownRenderer interface {
	Convert(source []byte, writer io.Writer, opts ...parser.ParseOption) error
}

func htmlEscape(val string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`"`, "&quot;",
		`'`, "&#39;",
		`<`, "&lt;",
		`>`, "&gt;",
	)
	return replacer.Replace(val)
}
