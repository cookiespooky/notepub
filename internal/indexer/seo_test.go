package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

const seoRules = `version: 1
fields:
  required: ["type", "slug", "title"]
types:
  home:
    template: "home.html"
    permalink: "/"
    include_in: { sitemap: true, search: false }
  post:
    template: "post.html"
    permalink: "/{{ slug }}/"
    include_in: { sitemap: true, search: true }
  page:
    template: "page.html"
    permalink: "/{{ slug }}/"
    include_in: { sitemap: true, search: true }
links:
  - name: "body"
    kind: "wikilinks"
    from_types: ["home", "post", "page"]
    to_types: ["home", "post", "page"]
    resolve: { order: ["filename", "slug"], ambiguity: "error", missing: "warn_skip", case: "insensitive" }
`

type seoSite struct {
	root string
	cfg  config.Config
}

func newSEOSite(t *testing.T, files map[string]string) seoSite {
	t.Helper()
	root := t.TempDir()
	content := filepath.Join(root, "content")
	for name, body := range files {
		p := filepath.Join(content, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rulesPath := filepath.Join(root, "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(seoRules), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Site: config.SiteConfig{
			ID: "t", BaseURL: "https://example.com", Title: "Пример", Description: "Сайт для проверки", Language: "ru",
		},
		Content:      config.ContentConfig{Source: "local", LocalDir: content},
		OGTypeByType: map[string]string{"post": "article", "page": "website"},
		Paths: config.PathsConfig{
			ArtifactsDir: filepath.Join(root, "artifacts"),
			SnapshotFile: filepath.Join(root, "snapshot", "objects.json"),
		},
		RulesPath: rulesPath,
	}
	return seoSite{root: root, cfg: cfg}
}

func (s seoSite) index(t *testing.T) models.ResolveIndex {
	t.Helper()
	if err := Run(context.Background(), s.cfg); err != nil {
		t.Fatalf("index: %v", err)
	}
	idx, err := loadResolve(filepath.Join(s.cfg.Paths.ArtifactsDir, resolveFileName))
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func (s seoSite) touchAll(t *testing.T, when time.Time) {
	t.Helper()
	filepath.Walk(s.cfg.Content.LocalDir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			os.Chtimes(p, when, when)
		}
		return nil
	})
}

func TestLastModifiedComesFromNoteBeforeFileTime(t *testing.T) {
	site := newSEOSite(t, map[string]string{
		"home.md": "---\ntype: home\nslug: home\ntitle: Главная\n---\n",
		"a.md":    "---\ntype: post\nslug: a\ntitle: А\nupdated: 2025-03-04\n---\n",
		"b.md":    "---\ntype: post\nslug: b\ntitle: Б\nlastmod: \"2025-05-06T07:08:09Z\"\n---\n",
		"c.md":    "---\ntype: post\nslug: c\ntitle: В\n---\n",
	})
	checkout := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	site.touchAll(t, checkout)
	idx := site.index(t)

	if got := idx.Routes["/a"].LastModified; got != "2025-03-04T00:00:00Z" {
		t.Errorf("updated: lastmod = %q", got)
	}
	if got := idx.Routes["/b"].LastModified; got != "2025-05-06T07:08:09Z" {
		t.Errorf("lastmod: lastmod = %q", got)
	}
	if got := idx.Routes["/c"].LastModified; got != checkout.Format(time.RFC3339) {
		t.Errorf("без даты в заметке ожидалось время файла, получено %q", got)
	}
	sitemap, err := os.ReadFile(filepath.Join(site.cfg.Paths.ArtifactsDir, "sitemap-0001.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sitemap), "<lastmod>2025-03-04</lastmod>") {
		t.Errorf("sitemap не взял дату из заметки:\n%s", sitemap)
	}

	// A second, incremental run reuses the meta of unchanged files and must
	// keep the note's date rather than fall back to the file time.
	idx = site.index(t)
	if got := idx.Routes["/a"].LastModified; got != "2025-03-04T00:00:00Z" {
		t.Errorf("повторная индексация потеряла дату из заметки: %q", got)
	}
}

func TestSEOTitleAlternatesAndAutoJSONLD(t *testing.T) {
	site := newSEOSite(t, map[string]string{
		"home.md": "---\ntype: home\nslug: home\ntitle: Главная\n---\n",
		"a.md": "---\ntype: post\nslug: a\ntitle: Длинный заголовок статьи\nseo_title: Короткий\ndescription: Описание\n" +
			"date: 2025-01-02\nauthor: Анна\nalternates:\n  en: /en/a/\n  x-default: /en/a/\n  ru: /a/\n---\n",
		"s.md":      "---\ntype: page\nslug: s\ntitle: Ремонт\nschema_type: Service\n---\n",
		"own.md":    "---\ntype: post\nslug: own\ntitle: Своя разметка\njsonld: {\"@type\": \"Thing\"}\n---\n",
		"hidden.md": "---\ntype: post\nslug: hidden\ntitle: Скрытая\nnoindex: true\n---\n",
	})
	site.cfg.SEO.JSONLD = "auto"
	idx := site.index(t)

	a := idx.Meta["/a"]
	if a.SEOTitle != "Короткий" {
		t.Errorf("SEOTitle = %q", a.SEOTitle)
	}
	if a.OpenGraph["title"] != "Короткий" {
		t.Errorf("og:title должен брать seo_title, получено %q", a.OpenGraph["title"])
	}
	want := []models.Alternate{
		{Lang: "en", URL: "https://example.com/en/a/"},
		{Lang: "ru", URL: "https://example.com/a/"},
		{Lang: "x-default", URL: "https://example.com/en/a/"},
	}
	if len(a.Alternates) != len(want) {
		t.Fatalf("alternates = %+v", a.Alternates)
	}
	for i := range want {
		if a.Alternates[i] != want[i] {
			t.Errorf("alternates[%d] = %+v, want %+v", i, a.Alternates[i], want[i])
		}
	}

	var doc struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal(a.JSONLD, &doc); err != nil {
		t.Fatalf("jsonld: %v (%s)", err, a.JSONLD)
	}
	if len(doc.Graph) != 3 || doc.Graph[2]["@type"] != "Article" {
		t.Fatalf("ожидались WebSite, WebPage и Article: %s", a.JSONLD)
	}
	art := doc.Graph[2]
	if art["datePublished"] != "2025-01-02T00:00:00Z" || art["headline"] != "Длинный заголовок статьи" {
		t.Errorf("Article: %v", art)
	}
	if author, _ := art["author"].(map[string]interface{}); author["name"] != "Анна" {
		t.Errorf("author: %v", art["author"])
	}
	if doc.Graph[0]["inLanguage"] != "ru" {
		t.Errorf("язык сайта не попал в разметку: %v", doc.Graph[0])
	}

	if !strings.Contains(compactJSON(t, idx.Meta["/s"].JSONLD), `"@type":"Service"`) {
		t.Errorf("schema_type не стал главной сущностью: %s", idx.Meta["/s"].JSONLD)
	}
	if strings.Contains(string(idx.Meta["/"].JSONLD), "Article") {
		t.Errorf("главная не статья: %s", idx.Meta["/"].JSONLD)
	}
	var own bytes.Buffer
	if err := json.Compact(&own, idx.Meta["/own"].JSONLD); err != nil || own.String() != `{"@type":"Thing"}` {
		t.Errorf("разметка из заметки должна остаться как есть, получено %s", idx.Meta["/own"].JSONLD)
	}
	if len(idx.Meta["/hidden"].JSONLD) != 0 {
		t.Errorf("закрытой от индекса странице разметка не нужна: %s", idx.Meta["/hidden"].JSONLD)
	}
}

func TestAutoJSONLDIsOffByDefault(t *testing.T) {
	site := newSEOSite(t, map[string]string{
		"home.md": "---\ntype: home\nslug: home\ntitle: Главная\n---\n",
	})
	idx := site.index(t)
	if len(idx.Meta["/"].JSONLD) != 0 {
		t.Errorf("без seo.jsonld: auto разметку пишет тема, движок — нет: %s", idx.Meta["/"].JSONLD)
	}
}

func TestConfigChangeReindexesUnchangedFiles(t *testing.T) {
	site := newSEOSite(t, map[string]string{
		"home.md": "---\ntype: home\nslug: home\ntitle: Главная\n---\n",
		"a.md":    "---\ntype: post\nslug: a\ntitle: А\n---\n",
	})
	site.index(t)
	site.cfg.Site.BaseURL = "https://new.example.org/docs"
	idx := site.index(t)
	if got := idx.Meta["/a"].Canonical; got != "https://new.example.org/docs/a/" {
		t.Errorf("после смены base_url canonical остался старым: %q", got)
	}
	if got := idx.Meta["/a"].OpenGraph["url"]; got != "https://new.example.org/docs/a/" {
		t.Errorf("og:url остался старым: %q", got)
	}
}

func TestPaginatedPageHasItsOwnOGURL(t *testing.T) {
	idx, cfg := paginationFixture(14)
	idx.Meta["/blog"] = models.MetaEntry{Type: "blog", Slug: "blog", Title: "Блог",
		OpenGraph: map[string]string{"url": "https://example.com/blog/"},
		JSONLD:    json.RawMessage(`{"@type":"WebPage"}`)}
	if err := synthesizePaginatedRoutes(&idx, cfg, "https://example.com", map[string]bool{"/blog": true}); err != nil {
		t.Fatal(err)
	}
	if got := idx.Meta["/blog/page/2"].OpenGraph["url"]; got != "https://example.com/blog/page/2/" {
		t.Errorf("og:url второй страницы = %q", got)
	}
	if got := idx.Meta["/blog"].OpenGraph["url"]; got != "https://example.com/blog/" {
		t.Errorf("первая страница изменилась вместе со второй: %q", got)
	}
	if len(idx.Meta["/blog/page/2"].JSONLD) != 0 {
		t.Error("сгенерированная разметка первой страницы не должна переходить на вторую")
	}
}

func TestCheckSEO(t *testing.T) {
	long := strings.Repeat("слово ", 15)
	site := newSEOSite(t, map[string]string{
		"home.md": "---\ntype: home\nslug: home\ntitle: Главная\ndescription: Главная страница сайта с описанием нужной длины для поиска\n---\n[[a]] [[b]]\n",
		"a.md":    "---\ntype: post\nslug: a\ntitle: Первая\nseo_title: Одинаковый\ndescription: Коротко\n---\n",
		"b.md":    "---\ntype: post\nslug: b\ntitle: Вторая\nseo_title: Одинаковый\n---\n",
		"c.md":    "---\ntype: post\nslug: c\ntitle: " + long + "\ndescription: Описание страницы, у которой нет ни одной ссылки ни туда, ни оттуда\n---\n",
		"d.md":    "---\ntype: post\nslug: d\ntitle: Черновик\ndraft: true\n---\n",
		"e.md":    "---\ntype: page\nslug: e\ntitle: Ещё\ndescription: Описание ещё одной страницы нужной длины для проверки поиска\n---\n[[home]]\n",
	})
	site.touchAll(t, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	site.cfg.Site.BaseURL = "https://example.com/np"
	idx := site.index(t)
	r, err := rules.Load(site.cfg.RulesPath)
	if err != nil {
		t.Fatal(err)
	}
	diags := CheckSEO(idx, site.cfg, r)
	has := func(code, path string) bool {
		for _, d := range diags {
			if d.Code == code && d.Path == path {
				return true
			}
		}
		return false
	}
	for _, c := range []struct{ code, path string }{
		{"title-duplicate", "/a"}, {"title-duplicate", "/b"},
		{"description-short", "/a"}, {"description-missing", "/b"},
		{"title-long", "/c"}, {"page-isolated", "/c"},
		{"robots-under-path", ""}, {"lastmod-uniform", ""},
	} {
		if !has(c.code, c.path) {
			t.Errorf("нет находки %s для %q; все находки: %+v", c.code, c.path, diags)
		}
	}
	for _, d := range diags {
		if d.Path == "/d" {
			t.Errorf("черновик не индексируется и не проверяется: %+v", d)
		}
		if d.Path == "/a" && d.Code == "page-isolated" {
			t.Errorf("на /a ссылается главная: %+v", d)
		}
	}
}

func compactJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatalf("jsonld не JSON: %v", err)
	}
	return b.String()
}
