package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/localutil"
	"github.com/cookiespooky/notepub/internal/mediautil"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/s3util"
)

// metaFormatVersion is part of the config fingerprint: bump it whenever
// buildMetaEntry starts writing something new, so an incremental index does
// not keep meta built by an older engine.
const metaFormatVersion = 2

// updatedKeys are the frontmatter keys that say when a page last changed.
var updatedKeys = []string{"updated", "lastmod", "modified", "date_modified"}

// publishedKeys say when a page first appeared.
var publishedKeys = []string{"date", "published", "date_published"}

type metaOptions struct {
	Site         config.SiteConfig
	OGTypeByType map[string]string
	SEO          config.SEOConfig
	Finder       *mediautil.Finder
}

func metaOptionsFromConfig(cfg config.Config) metaOptions {
	return metaOptions{Site: cfg.Site, OGTypeByType: cfg.OGTypeByType, SEO: cfg.SEO}
}

// fmTime reads the first of keys that holds a date. YAML gives an unquoted
// date as time.Time and a quoted one as a string; both are accepted.
func fmTime(meta map[string]interface{}, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		val, ok := meta[key]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case time.Time:
			return v.UTC(), true
		case string:
			if t, ok := parseLooseTime(v); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func parseLooseTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// effectiveLastModified is the date a page changed: the frontmatter wins over
// the file's modification time, which in CI is the checkout time of every file.
func effectiveLastModified(meta map[string]interface{}, fileLM string) string {
	if t, ok := fmTime(meta, updatedKeys...); ok {
		return t.Format(time.RFC3339)
	}
	return fileLM
}

// buildAlternates turns `alternates: {en: /en/page/, ru: /ru/page/}` into
// absolute URLs, sorted by language with x-default last.
func buildAlternates(meta map[string]interface{}, baseURL string) []models.Alternate {
	raw := mapFromMeta(meta, "alternates")
	if len(raw) == 0 {
		return nil
	}
	out := make([]models.Alternate, 0, len(raw))
	for lang, href := range raw {
		lang = strings.TrimSpace(lang)
		href = strings.TrimSpace(href)
		if lang == "" || href == "" {
			continue
		}
		u := href
		if !isExternalURL(href) {
			u = buildAbsoluteURL(baseURL, href)
		}
		out = append(out, models.Alternate{Lang: lang, URL: u})
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Lang == "x-default") != (out[j].Lang == "x-default") {
			return out[j].Lang == "x-default"
		}
		return out[i].Lang < out[j].Lang
	})
	return out
}

func isArticleType(typeName string, opts metaOptions) bool {
	if len(opts.SEO.ArticleTypes) > 0 {
		for _, t := range opts.SEO.ArticleTypes {
			if t == typeName {
				return true
			}
		}
		return false
	}
	return opts.OGTypeByType[typeName] == "article"
}

// buildAutoJSONLD writes schema.org markup for a page whose frontmatter has
// none: WebSite and WebPage always, Article for article types, and the type a
// note names in schema_type (Service, FAQPage, Person…) as the main entity.
func buildAutoJSONLD(entry models.MetaEntry, meta map[string]interface{}, pathVal, lastModified string, opts metaOptions) json.RawMessage {
	base := strings.TrimRight(opts.Site.BaseURL, "/")
	lang := opts.Site.Language
	siteID := base + "/#website"
	pageURL := entry.Canonical
	if pageURL == "" {
		pageURL = base + "/"
	}
	website := map[string]interface{}{
		"@type": "WebSite",
		"@id":   siteID,
		"url":   base + "/",
		"name":  opts.Site.Title,
	}
	if lang != "" {
		website["inLanguage"] = lang
	}
	page := map[string]interface{}{
		"@type":    "WebPage",
		"@id":      pageURL + "#webpage",
		"url":      pageURL,
		"name":     firstNonEmpty(entry.SEOTitle, entry.Title),
		"isPartOf": map[string]string{"@id": siteID},
	}
	if entry.Description != "" {
		page["description"] = entry.Description
	}
	if lang != "" {
		page["inLanguage"] = lang
	}
	if entry.Image != "" {
		page["primaryImageOfPage"] = map[string]string{"@type": "ImageObject", "url": entry.Image}
	}
	if t, err := time.Parse(time.RFC3339, lastModified); err == nil {
		page["dateModified"] = t.UTC().Format(time.RFC3339)
	}
	graph := []interface{}{website, page}

	schemaType := strings.TrimSpace(stringFromMeta(meta, "schema_type"))
	if schemaType == "" && pathVal != "/" && isArticleType(entry.Type, opts) {
		schemaType = "Article"
	}
	if schemaType != "" {
		main := map[string]interface{}{
			"@type":            schemaType,
			"mainEntityOfPage": map[string]string{"@id": pageURL + "#webpage"},
		}
		name := firstNonEmpty(entry.Title, entry.SEOTitle)
		if isArticleSchema(schemaType) {
			main["headline"] = truncateRunes(name, 110)
			if entry.Published != "" {
				main["datePublished"] = entry.Published
			}
			if mod, ok := page["dateModified"]; ok {
				main["dateModified"] = mod
			}
			if author := firstNonEmpty(stringFromMeta(meta, "author"), opts.SEO.AuthorName); author != "" {
				main["author"] = map[string]string{"@type": "Person", "name": author}
			}
		} else {
			main["name"] = name
			main["url"] = pageURL
		}
		if entry.Description != "" {
			main["description"] = entry.Description
		}
		if entry.Image != "" {
			main["image"] = entry.Image
		}
		if lang != "" {
			main["inLanguage"] = lang
		}
		graph = append(graph, main)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"@context": "https://schema.org",
		"@graph":   graph,
	})
	if err != nil {
		return nil
	}
	return payload
}

func isArticleSchema(t string) bool {
	switch t {
	case "Article", "BlogPosting", "NewsArticle", "TechArticle":
		return true
	}
	return false
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// configFingerprint changes whenever something buildMetaEntry reads from the
// config or the rules changes. An incremental index reuses meta only for files
// that did not change, so without this a new base_url or language would keep
// the old canonical and og:url on every page that was not edited.
func configFingerprint(cfg config.Config, mediaFiles []string) string {
	rulesBytes, _ := os.ReadFile(cfg.RulesPath)
	payload, _ := json.Marshal(struct {
		Version int
		Site    config.SiteConfig
		OG      map[string]string
		SEO     config.SEOConfig
		Prefix  string
		Rules   string
		Media   []string
	}{metaFormatVersion, cfg.Site, cfg.OGTypeByType, cfg.SEO, cfg.S3.Prefix, string(rulesBytes), mediaFiles})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// listMediaFiles collects every file the build publishes under /media/: what
// is not a note in the content folder (or bucket), and the files of media.dirs.
func listMediaFiles(cfg config.Config, objects []s3util.Object) ([]string, error) {
	seen := map[string]bool{}
	add := func(key string) {
		key = strings.TrimPrefix(key, "/")
		if key != "" {
			seen[key] = true
		}
	}
	prefix := strings.TrimPrefix(cfg.S3.Prefix, "/")
	switch cfg.Content.Source {
	case "local":
		files, err := localutil.ListMediaFiles(cfg.Content.LocalDir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			add(strings.TrimPrefix(f, prefix))
		}
	case "s3":
		for _, obj := range objects {
			if strings.HasSuffix(strings.ToLower(obj.Key), ".md") {
				continue
			}
			add(strings.TrimPrefix(obj.Key, prefix))
		}
	}
	for _, dir := range cfg.Media.Dirs {
		files, err := localutil.ListMediaFiles(dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			add(f)
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
