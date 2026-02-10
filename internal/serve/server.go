package serve

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/yuin/goldmark"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/localutil"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/s3util"
	"github.com/cookiespooky/notepub/internal/urlutil"
)

const (
	resolveTimeout = 2 * time.Second
	presignTimeout = 2 * time.Second
	fetchTimeout   = 8 * time.Second
	maxMarkdown    = 5 * 1024 * 1024
)

var (
	metricRequestsTotal = expvar.NewInt("notepub_requests_total")
	metric2xx           = expvar.NewInt("notepub_responses_2xx")
	metric3xx           = expvar.NewInt("notepub_responses_3xx")
	metric4xx           = expvar.NewInt("notepub_responses_4xx")
	metric5xx           = expvar.NewInt("notepub_responses_5xx")
	metricCacheHit      = expvar.NewInt("notepub_cache_hit")
	metricCacheMiss     = expvar.NewInt("notepub_cache_miss")
	metricCacheStale    = expvar.NewInt("notepub_cache_stale")
)

type Server struct {
	cfg      config.Config
	store    *ResolveStore
	cache    *HtmlCache
	theme    *Theme
	s3client *s3.Client
	md       goldmark.Markdown
	rules    rules.Rules
}

func New(cfg config.Config, store *ResolveStore, cache *HtmlCache, theme *Theme, s3client *s3.Client, rulesCfg rules.Rules) *Server {
	md := newMarkdownRenderer()
	return &Server{
		cfg:      cfg,
		store:    store,
		cache:    cache,
		theme:    theme,
		s3client: s3client,
		md:       md,
		rules:    rulesCfg,
	}
}

func (s *Server) Router() http.Handler {
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metricRequestsTotal.Add(1)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	switch {
	case r.URL.Path == "/health":
		s.handleHealth(rec, r)
	case r.URL.Path == "/metrics":
		expvar.Handler().ServeHTTP(rec, r)
	case r.URL.Path == "/robots.txt":
		s.handleRobots(rec, r)
	case strings.HasPrefix(r.URL.Path, "/sitemap"):
		s.handleSitemap(rec, r)
	case strings.HasPrefix(r.URL.Path, "/assets/"):
		s.handleAssets(rec, r)
	case strings.HasPrefix(r.URL.Path, "/media/"):
		s.handleMedia(rec, r)
	case strings.HasPrefix(r.URL.Path, "/v1/search"):
		s.handleSearch(rec, r)
	case r.URL.Path == "/search":
		s.handleSearchPage(rec, r)
	case r.URL.Path == "/favicon.ico":
		s.handleFavicon(rec, r)
	default:
		s.handlePage(rec, r)
	}
	trackStatus(rec.status)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.cfg.Paths.ArtifactsDir, "robots.txt")
	serveFile(w, r, path, "text/plain")
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path)
	if !strings.HasPrefix(name, "sitemap") {
		name = "sitemap-index.xml"
	}
	path := filepath.Join(s.cfg.Paths.ArtifactsDir, name)
	serveFile(w, r, path, "application/xml")
}

func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	if !isSafeAsset(name) {
		http.NotFound(w, r)
		return
	}
	if served := serveThemeAsset(w, r, s.theme, name); served {
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/media/")
	key, err := url.PathUnescape(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	key = strings.TrimPrefix(key, "/")
	if !isSafeKey(key) {
		http.NotFound(w, r)
		return
	}
	allowKey := key
	if prefix := s.cfg.S3.Prefix; prefix != "" && strings.HasPrefix(allowKey, prefix) {
		allowKey = strings.TrimPrefix(allowKey, prefix)
		allowKey = strings.TrimPrefix(allowKey, "/")
	}
	if !s.store.MediaAllowed(allowKey) {
		http.NotFound(w, r)
		return
	}

	if s.cfg.Content.Source == "local" {
		if prefix := s.cfg.S3.Prefix; prefix != "" && !strings.HasPrefix(key, prefix) {
			key = path.Join(prefix, key)
		}
		localPath, err := localutil.ResolvePath(s.cfg.Content.LocalDir, key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		serveFile(w, r, localPath, "")
		return
	}

	if prefix := s.cfg.S3.Prefix; prefix != "" && !strings.HasPrefix(key, prefix) {
		key = path.Join(prefix, key)
	}
	if s.cfg.S3.Anonymous {
		fetchCtx, cancelFetch := context.WithTimeout(r.Context(), fetchTimeout)
		defer cancelFetch()
		resp, err := s.s3client.GetObject(fetchCtx, &s3.GetObjectInput{
			Bucket: &s.cfg.S3.Bucket,
			Key:    &key,
		})
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer resp.Body.Close()
		if resp.ContentType != nil && *resp.ContentType != "" {
			w.Header().Set("Content-Type", *resp.ContentType)
		}
		if resp.ContentLength != nil && *resp.ContentLength > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
		}
		_, _ = io.Copy(w, resp.Body)
		return
	}
	psURL, _, err := s3util.PresignGet(r.Context(), s.s3client, s.cfg.S3.Bucket, key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, psURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	if served := serveThemeAsset(w, r, s.theme, "favicon.ico"); served {
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 8
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}
	cursor := r.URL.Query().Get("cursor")
	if len(q) < 2 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items":       []SearchItem{},
			"next_cursor": "",
		})
		return
	}
	items, nextCursor, err := s.store.Search(q, limit, cursor)
	if err != nil {
		http.Error(w, "search unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

func (s *Server) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	host := stripPort(r.Host)
	if !s.hostAllowed(host) {
		s.renderNotFound(w, r)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	cursor := r.URL.Query().Get("cursor")
	limit := 10
	items := []SearchItem{}
	nextCursor := ""
	if len(q) >= 2 {
		if res, next, err := s.store.Search(q, limit, cursor); err == nil {
			items = res
			nextCursor = next
		}
	}

	title := "Search"
	if q != "" {
		title = "Search: " + q
	}
	meta := models.MetaEntry{
		Title:     title,
		Canonical: buildSearchCanonical(s.cfg.Site.BaseURL, q, cursor),
		Robots:    "noindex, follow",
	}
	data := buildPageData(meta, "", s.cfg.Site.BaseURL)
	data.IsSearch = true
	data.SearchMode = "server"
	data.SearchQuery = q
	data.SearchItems = items
	data.SearchNextCursor = nextCursor
	resolveCtx, cancel := context.WithTimeout(r.Context(), resolveTimeout)
	defer cancel()
	if idx, _, err := s.store.GetWithWikiMapContext(resolveCtx); err == nil {
		data.Collections = buildCollections(idx, s.rules, "/search")
	}

	rendered, err := s.theme.RenderPage(data)
	if err != nil {
		if html, renderErr := s.theme.RenderError(err, data); renderErr == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(html))
		} else {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	host := stripPort(r.Host)
	if !s.hostAllowed(host) {
		s.renderNotFound(w, r)
		return
	}
	pathVal := r.URL.Path
	if pathVal == "" {
		pathVal = "/"
	}

	resolveCtx, cancel := context.WithTimeout(r.Context(), resolveTimeout)
	defer cancel()
	idx, wikiMap, err := s.store.GetWithWikiMapContext(resolveCtx)
	if err != nil {
		s.serveStaleOr503(w, r, pathVal)
		return
	}
	route, ok := idx.Routes[pathVal]
	if !ok {
		s.renderNotFound(w, r)
		return
	}
	if route.Status == 301 && route.RedirectTo != "" {
		http.Redirect(w, r, route.RedirectTo, http.StatusMovedPermanently)
		return
	}
	if route.Status != 200 {
		s.renderNotFound(w, r)
		return
	}

	if inm := r.Header.Get("If-None-Match"); inm != "" && inm == route.RouteETag {
		s.writePageHeaders(w, route, "hit", false)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	cacheHTML, cacheStatus, err := s.cache.Read(s.cfg.Site.ID, pathVal, route.RouteETag)
	if err == nil && cacheHTML != "" && cacheStatus == "hit" {
		s.writePage(w, pathVal, idx, route, cacheHTML, "hit", false)
		return
	}

	if route.S3Key == "" {
		s.serveStaleOr503(w, r, pathVal)
		return
	}
	var markdown string
	if s.cfg.Content.Source == "local" {
		localPath, err := localutil.ResolvePath(s.cfg.Content.LocalDir, route.S3Key)
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		file, err := os.Open(localPath)
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		defer file.Close()
		body, err := readMarkdownLimited(file)
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		markdown = body
	} else if s.cfg.S3.Anonymous {
		fetchCtx, cancelFetch := context.WithTimeout(r.Context(), fetchTimeout)
		defer cancelFetch()
		resp, err := s.s3client.GetObject(fetchCtx, &s3.GetObjectInput{
			Bucket: &s.cfg.S3.Bucket,
			Key:    &route.S3Key,
		})
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		defer resp.Body.Close()
		body, err := readMarkdownLimited(resp.Body)
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		markdown = body
	} else {
		psCtx, cancelPresign := context.WithTimeout(r.Context(), presignTimeout)
		defer cancelPresign()
		psURL, _, err := s3util.PresignGet(psCtx, s.s3client, s.cfg.S3.Bucket, route.S3Key)
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		fetchCtx, cancelFetch := context.WithTimeout(r.Context(), fetchTimeout)
		defer cancelFetch()
		body, err := fetchPresigned(fetchCtx, psURL)
		if err != nil {
			s.serveStaleOr503(w, r, pathVal)
			return
		}
		markdown = body
	}

	htmlBody, err := s.renderMarkdown(markdown, route.S3Key, wikiMap)
	if err != nil {
		s.serveStaleOr503(w, r, pathVal)
		return
	}
	_ = s.cache.Write(s.cfg.Site.ID, pathVal, route.RouteETag, htmlBody)
	s.writePage(w, pathVal, idx, route, htmlBody, "miss", false)
}

func (s *Server) renderMarkdown(markdown string, baseKey string, wikiMap map[string]string) (string, error) {
	markdown = normalizeMarkdownImages(markdown, baseKey, s.cfg.S3.Prefix, s.cfg.Site.MediaBaseURL)
	markdown = normalizeMarkdownLinks(markdown, wikiMap)
	var buf strings.Builder
	if err := s.md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *Server) writePage(w http.ResponseWriter, pathVal string, idx models.ResolveIndex, route models.RouteEntry, body string, cacheStatus string, stale bool) {
	s.writePageHeaders(w, route, cacheStatus, stale)
	meta := idx.Meta[pathVal]
	data := buildPageData(meta, body, s.cfg.Site.BaseURL)
	data.Template = s.templateForType(meta.Type)
	data.Page.NoIndex = route.NoIndex
	data.SearchMode = "server"
	if pathVal == "/" {
		data.IsHome = true
	}
	data.Collections = buildCollections(idx, s.rules, pathVal)
	rendered, err := s.theme.RenderPage(data)
	if err != nil {
		if html, renderErr := s.theme.RenderError(err, data); renderErr == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(html))
		} else {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

func buildPageData(meta models.MetaEntry, body, baseURL string) PageData {
	data := PageData{
		Title:      meta.Title,
		Canonical:  meta.Canonical,
		BaseURL:    baseURL,
		AssetsBase: urlutil.JoinBaseURL(baseURL, "/assets"),
		Meta:       MetaData{Robots: meta.Robots},
		Body:       template.HTML(body),
		Page: PageInfo{
			Type:        meta.Type,
			Slug:        meta.Slug,
			Title:       meta.Title,
			Description: meta.Description,
			Canonical:   meta.Canonical,
			Category:    meta.Category,
		},
		Core: CoreFields{
			Type:        meta.Type,
			Slug:        meta.Slug,
			Title:       meta.Title,
			Description: meta.Description,
		},
		FM: meta.FM,
	}
	if len(meta.OpenGraph) > 0 {
		for k, v := range meta.OpenGraph {
			data.Meta.OpenGraph = append(data.Meta.OpenGraph, MetaKV{Key: "og:" + k, Value: v})
		}
	}
	if len(meta.JSONLD) > 0 {
		if json.Valid(meta.JSONLD) {
			data.Meta.JSONLD = template.JS(meta.JSONLD)
		}
	}
	return data
}

func (s *Server) templateForType(typeName string) string {
	if typeName == "" {
		return ""
	}
	td, ok := s.rules.Types[typeName]
	if !ok {
		return ""
	}
	if td.Template != "" {
		return td.Template
	}
	return ""
}

func (s *Server) writePageHeaders(w http.ResponseWriter, route models.RouteEntry, cacheStatus string, stale bool) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, s-maxage=%d, stale-if-error=%d", s.cfg.Cache.HTMLTTLSeconds, s.cfg.Cache.StaleIfErrorSeconds))
	w.Header().Set("X-Notepub-Cache", cacheStatus)
	switch cacheStatus {
	case "hit":
		metricCacheHit.Add(1)
	case "miss":
		metricCacheMiss.Add(1)
	case "stale":
		metricCacheStale.Add(1)
	}
	if route.RouteETag != "" {
		w.Header().Set("ETag", route.RouteETag)
	}
	if stale {
		w.Header().Set("X-Index-Stale", "true")
		w.Header().Set("Warning", "110 - Response is stale")
	}
}

func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	html, _ := s.theme.RenderNotFound(s.cfg.Site.BaseURL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(html))
}

func (s *Server) serveStaleOr503(w http.ResponseWriter, r *http.Request, routePath string) {
	if stale, _, err := s.cache.Read(s.cfg.Site.ID, routePath, ""); err == nil && stale != "" {
		w.Header().Set("X-Notepub-Cache", "stale")
		w.Header().Set("X-Index-Stale", "true")
		w.Header().Set("Warning", "110 - Response is stale")
		metricCacheStale.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(stale))
		return
	}
	w.Header().Set("Retry-After", "60")
	http.Error(w, "Index temporarily unavailable", http.StatusServiceUnavailable)
}

func (s *Server) hostAllowed(host string) bool {
	if s.cfg.Site.Host == "" {
		return true
	}
	if strings.EqualFold(host, s.cfg.Site.Host) {
		return true
	}
	for _, alias := range s.cfg.Site.HostAliases {
		if strings.EqualFold(host, alias) {
			return true
		}
	}
	return false
}

func stripPort(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return hostport
}

func fetchPresigned(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("fetch presigned status %d", res.StatusCode)
	}
	return readMarkdownLimited(res.Body)
}

func readMarkdownLimited(r io.Reader) (string, error) {
	limited := io.LimitReader(r, maxMarkdown+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(b)) > maxMarkdown {
		return "", fmt.Errorf("markdown too large")
	}
	return string(b), nil
}

func serveThemeAsset(w http.ResponseWriter, r *http.Request, theme *Theme, name string) bool {
	assetFS := theme.AssetFS()
	path := filepath.ToSlash(filepath.Join(theme.assetsSubdir, name))
	file, err := assetFS.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	if stat.IsDir() {
		return false
	}
	if ctype := mime.TypeByExtension(filepath.Ext(name)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	io.Copy(w, file)
	return true
}

func serveFile(w http.ResponseWriter, r *http.Request, path string, fallbackType string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if ctype := mime.TypeByExtension(filepath.Ext(path)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	} else if fallbackType != "" {
		w.Header().Set("Content-Type", fallbackType)
	}
	http.ServeFile(w, r, path)
}

func isSafeAsset(name string) bool {
	if name == "" {
		return false
	}
	clean := filepath.Clean(name)
	if strings.Contains(clean, "..") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") {
		return false
	}
	return true
}

func isSafeKey(key string) bool {
	if key == "" {
		return false
	}
	clean := path.Clean("/" + key)
	if strings.Contains(clean, "..") || clean == "/" {
		return false
	}
	return !strings.HasPrefix(clean, "/..")
}

func buildAbsoluteURL(baseURL, p string) string {
	return urlutil.JoinBaseURL(baseURL, p)
}

func buildSearchCanonical(baseURL, q, cursor string) string {
	base := urlutil.JoinBaseURL(baseURL, "/search")
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	if q == "" {
		return u.String()
	}
	v := url.Values{}
	v.Set("q", q)
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	u.RawQuery = v.Encode()
	return u.String()
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func trackStatus(status int) {
	switch {
	case status >= 200 && status < 300:
		metric2xx.Add(1)
	case status >= 300 && status < 400:
		metric3xx.Add(1)
	case status >= 400 && status < 500:
		metric4xx.Add(1)
	case status >= 500:
		metric5xx.Add(1)
	}
}
