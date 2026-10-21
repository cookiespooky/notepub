package serve

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/urlutil"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language,omitempty"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	AtomLink      rssAtom   `xml:"atom:link"`
	Items         []rssItem `xml:"item"`
}

type rssAtom struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate,omitempty"`
	Description string  `xml:"description,omitempty"`
}

type rssGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink bool   `xml:"isPermaLink,attr"`
}

type feedEntry struct {
	meta models.MetaEntry
	when time.Time
	path string
}

// feedTypes are the content types that go into the feed: feed.types, else
// every type og_type_by_type calls an article.
func feedTypes(cfg config.Config) map[string]bool {
	out := map[string]bool{}
	for _, t := range cfg.Feed.Types {
		out[t] = true
	}
	if len(out) == 0 {
		for t, og := range cfg.OGTypeByType {
			if og == "article" {
				out[t] = true
			}
		}
	}
	return out
}

// RenderFeed builds the RSS document from the index. Its bytes depend only on
// the content: lastBuildDate is the newest item's date, not the build time.
func RenderFeed(idx models.ResolveIndex, cfg config.Config, rulesCfg rules.Rules) ([]byte, bool) {
	if !cfg.Feed.Enabled {
		return nil, false
	}
	types := feedTypes(cfg)
	entries := []feedEntry{}
	for p, route := range idx.Routes {
		if route.Status != http.StatusOK || route.NoIndex || route.PageNum > 0 {
			continue
		}
		meta, ok := idx.Meta[p]
		if !ok || !types[meta.Type] || boolFromMeta(meta.FM, "draft") {
			continue
		}
		when := parseRFC3339(meta.Published)
		if when.IsZero() {
			when = parseRFC3339(route.LastModified)
		}
		entries = append(entries, feedEntry{meta: meta, when: when, path: p})
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].when.Equal(entries[j].when) {
			return entries[i].when.After(entries[j].when)
		}
		return entries[i].path < entries[j].path
	})
	if len(entries) > cfg.Feed.Limit {
		entries = entries[:cfg.Feed.Limit]
	}

	base := strings.TrimRight(cfg.Site.BaseURL, "/")
	channel := rssChannel{
		Title:       firstNonEmptyString(cfg.Feed.Title, cfg.Site.Title),
		Link:        base + "/",
		Description: firstNonEmptyString(cfg.Feed.Description, cfg.Site.Description, cfg.Site.Title),
		Language:    cfg.Site.Language,
		AtomLink: rssAtom{
			Href: urlutil.JoinBaseURL(cfg.Site.BaseURL, "/"+cfg.Feed.Path),
			Rel:  "self",
			Type: "application/rss+xml",
		},
	}
	for i, e := range entries {
		link := e.meta.Canonical
		if link == "" {
			link = base + e.path
		}
		item := rssItem{
			Title:       firstNonEmptyString(e.meta.Title, e.meta.SEOTitle),
			Link:        link,
			GUID:        rssGUID{Value: link, IsPermaLink: true},
			Description: e.meta.Description,
		}
		if !e.when.IsZero() {
			item.PubDate = e.when.UTC().Format(time.RFC1123Z)
			if i == 0 {
				channel.LastBuildDate = item.PubDate
			}
		}
		channel.Items = append(channel.Items, item)
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(rssFeed{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Channel: channel}); err != nil {
		return nil, false
	}
	buf.WriteString("\n")
	return buf.Bytes(), true
}

func parseRFC3339(v string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
	if err != nil {
		return time.Time{}
	}
	return t
}
