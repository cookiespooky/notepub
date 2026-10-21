package serve

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

func feedFixture() (models.ResolveIndex, config.Config) {
	idx := models.ResolveIndex{
		Routes: map[string]models.RouteEntry{
			"/":           {Status: 200},
			"/old":        {Status: 200, LastModified: "2025-01-01T00:00:00Z"},
			"/new":        {Status: 200},
			"/draft":      {Status: 200},
			"/hidden":     {Status: 200, NoIndex: true},
			"/about":      {Status: 200},
			"/new/page/2": {Status: 200, PageNum: 2},
		},
		Meta: map[string]models.MetaEntry{
			"/":           {Type: "home", Title: "Блог"},
			"/old":        {Type: "post", Title: "Старый", Canonical: "https://ex.com/old/"},
			"/new":        {Type: "post", Title: "Новый", Description: "Свежий пост", Canonical: "https://ex.com/new/", Published: "2025-06-01T00:00:00Z"},
			"/draft":      {Type: "post", Title: "Черновик", FM: map[string]interface{}{"draft": true}},
			"/hidden":     {Type: "post", Title: "Скрытый"},
			"/about":      {Type: "page", Title: "Обо мне"},
			"/new/page/2": {Type: "post", Title: "Новый"},
		},
	}
	cfg := config.Config{
		Site:         config.SiteConfig{BaseURL: "https://ex.com", Title: "Блог", Language: "ru"},
		OGTypeByType: map[string]string{"post": "article", "page": "website"},
		Feed:         config.FeedConfig{Enabled: true, Limit: 20, Path: "feed.xml"},
	}
	return idx, cfg
}

func TestRenderFeed(t *testing.T) {
	idx, cfg := feedFixture()
	out, ok := RenderFeed(idx, cfg, rules.Rules{})
	if !ok {
		t.Fatal("лента не собрана")
	}
	var doc rssFeed
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("лента не XML: %v\n%s", err, out)
	}
	items := doc.Channel.Items
	if len(items) != 2 {
		t.Fatalf("в ленте только опубликованные посты, получено %d: %s", len(items), out)
	}
	if items[0].Title != "Новый" || items[1].Title != "Старый" {
		t.Errorf("порядок — от нового к старому: %q, %q", items[0].Title, items[1].Title)
	}
	if doc.Channel.LastBuildDate != items[0].PubDate {
		t.Errorf("lastBuildDate берётся из самого нового поста, а не из времени сборки: %q", doc.Channel.LastBuildDate)
	}
	if doc.Channel.Language != "ru" || !strings.Contains(string(out), `href="https://ex.com/feed.xml"`) {
		t.Errorf("язык или ссылка на саму ленту: %s", out)
	}
	again, _ := RenderFeed(idx, cfg, rules.Rules{})
	if string(again) != string(out) {
		t.Error("две сборки по одному содержимому дают разную ленту")
	}
}

func TestRenderFeedOffAndLimit(t *testing.T) {
	idx, cfg := feedFixture()
	cfg.Feed.Enabled = false
	if _, ok := RenderFeed(idx, cfg, rules.Rules{}); ok {
		t.Error("выключенная лента собралась")
	}
	cfg.Feed.Enabled = true
	cfg.Feed.Limit = 1
	cfg.Feed.Types = []string{"page"}
	out, _ := RenderFeed(idx, cfg, rules.Rules{})
	var doc rssFeed
	xml.Unmarshal(out, &doc)
	if len(doc.Channel.Items) != 1 || doc.Channel.Items[0].Title != "Обо мне" {
		t.Errorf("feed.types и limit не учтены: %s", out)
	}
}

func TestPageDataCarriesSEOFields(t *testing.T) {
	cfg := config.Config{Site: config.SiteConfig{BaseURL: "https://ex.com/np"}, Feed: config.FeedConfig{Enabled: true, Path: "feed.xml"}}
	meta := models.MetaEntry{Title: "Длинный", SEOTitle: "Короткий",
		Alternates: []models.Alternate{{Lang: "en", URL: "https://ex.com/np/en/"}}}
	data := buildPageData(meta, "", cfg)
	if data.Meta.Title != "Короткий" || data.Title != "Длинный" {
		t.Errorf("Meta.Title = %q, Title = %q", data.Meta.Title, data.Title)
	}
	if len(data.Meta.Alternates) != 1 {
		t.Errorf("alternates не дошли до шаблона")
	}
	if data.FeedURL != "https://ex.com/np/feed.xml" {
		t.Errorf("FeedURL = %q", data.FeedURL)
	}
	data = buildPageData(models.MetaEntry{Title: "Только"}, "", cfg)
	if data.Meta.Title != "Только" {
		t.Errorf("без seo_title Meta.Title = %q", data.Meta.Title)
	}
}
