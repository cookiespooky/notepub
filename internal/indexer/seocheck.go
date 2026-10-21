package indexer

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

// Limits are the usual ones search engines cut snippets at; they are
// warnings, not errors: a long title still works, it is only shortened.
const (
	seoTitleMax       = 70
	seoDescriptionMin = 50
	seoDescriptionMax = 200
)

// SEODiagnostic is one finding of `validate --seo`. Path is the page's route,
// or empty for a finding about the whole site.
type SEODiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

// indexablePages are the routes a search engine is asked to index: the ones
// that end up in the sitemap.
func indexablePages(idx models.ResolveIndex, cfg rules.Rules) []string {
	out := []string{}
	for p, rt := range idx.Routes {
		if rt.Status != 200 || rt.NoIndex || rt.PageNum > 0 {
			continue
		}
		meta, ok := idx.Meta[p]
		if !ok {
			continue
		}
		if td, ok := cfg.Types[meta.Type]; ok && !td.IncludeIn.Sitemap {
			continue
		}
		if boolFromMeta(meta.FM, "draft") || boolFromMeta(meta.FM, "noindex") {
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// CheckSEO looks for what costs a page its place in search results and can be
// fixed in the note: missing or repeated titles and descriptions, pages no
// other page links to, and two site-wide traps.
func CheckSEO(idx models.ResolveIndex, cfg config.Config, rulesCfg rules.Rules) []SEODiagnostic {
	pages := indexablePages(idx, rulesCfg)
	diags := []SEODiagnostic{}
	warn := func(code, path, msg string) {
		diags = append(diags, SEODiagnostic{Code: code, Severity: "warning", Path: path, Message: msg})
	}

	titles := map[string][]string{}
	descriptions := map[string][]string{}
	for _, p := range pages {
		meta := idx.Meta[p]
		title := strings.TrimSpace(firstNonEmpty(meta.SEOTitle, meta.Title))
		if title == "" {
			warn("title-missing", p, "no title")
		} else {
			if n := utf8.RuneCountInString(title); n > seoTitleMax {
				warn("title-long", p, fmt.Sprintf("title is %d characters; search results show about %d (shorten it or add seo_title)", n, seoTitleMax))
			}
			titles[strings.ToLower(title)] = append(titles[strings.ToLower(title)], p)
		}
		desc := strings.TrimSpace(meta.Description)
		switch n := utf8.RuneCountInString(desc); {
		case n == 0:
			warn("description-missing", p, "no description: search engines will pick a fragment of the text themselves")
		case n < seoDescriptionMin:
			warn("description-short", p, fmt.Sprintf("description is %d characters; %d–%d works best", n, seoDescriptionMin, seoDescriptionMax))
		case n > seoDescriptionMax:
			warn("description-long", p, fmt.Sprintf("description is %d characters and will be cut at about %d", n, seoDescriptionMax))
		}
		if desc != "" {
			descriptions[strings.ToLower(desc)] = append(descriptions[strings.ToLower(desc)], p)
		}
	}
	for _, group := range sortedGroups(titles) {
		for _, p := range group {
			warn("title-duplicate", p, "same title as "+strings.Join(others(group, p), ", "))
		}
	}
	for _, group := range sortedGroups(descriptions) {
		for _, p := range group {
			warn("description-duplicate", p, "same description as "+strings.Join(others(group, p), ", "))
		}
	}

	// A page with no link in either direction is reachable only through the
	// sitemap. Field links count: a page that names its hub is listed there.
	linked := map[string]bool{}
	for from, byRule := range idx.Links {
		for _, targets := range byRule {
			for _, to := range targets {
				if to != from {
					linked[from] = true
					linked[to] = true
				}
			}
		}
	}
	// Only notes are seen here, not the theme: a page the menu links to looks
	// isolated too. Section pages (a type with one page, like a blog index) are
	// menu items as a rule and are skipped; the rest is information, not a warning.
	if len(idx.Links) > 0 {
		for _, p := range pages {
			if p == "/" || linked[p] {
				continue
			}
			if _, single := rulesCfg.Validation.SinglePageOfType[idx.Meta[p].Type]; single {
				continue
			}
			diags = append(diags, SEODiagnostic{Code: "page-isolated", Severity: "info", Path: p,
				Message: "no note links here and it links to no note: unless the theme's menu leads here, only the sitemap does"})
		}
	}

	if base, err := url.Parse(cfg.Site.BaseURL); err == nil && base.Host != "" {
		if path := strings.Trim(base.Path, "/"); path != "" && !isLocalHost(base.Hostname()) {
			warn("robots-under-path", "",
				fmt.Sprintf("the site lives under /%s/, but crawlers read robots.txt only at %s://%s/robots.txt: add \"Sitemap: %s/sitemap.xml\" there", path, base.Scheme, base.Host, strings.TrimRight(cfg.Site.BaseURL, "/")))
		}
		for _, p := range pages {
			c, err := url.Parse(idx.Meta[p].Canonical)
			if err == nil && c.Host != "" && !strings.EqualFold(c.Host, base.Host) {
				diags = append(diags, SEODiagnostic{Code: "canonical-offsite", Severity: "info", Path: p,
					Message: "canonical points to " + c.Host + ": this page gives its ranking to that site"})
			}
		}
	}

	// Every page changed on the same day and no note says otherwise: the dates
	// are file times of a fresh checkout, and search engines stop trusting them.
	if len(pages) >= 5 {
		days := map[string]bool{}
		fromNotes := false
		for _, p := range pages {
			days[firstN(idx.Routes[p].LastModified, 10)] = true
			if _, ok := fmTime(idx.Meta[p].FM, updatedKeys...); ok {
				fromNotes = true
			}
		}
		if len(days) == 1 && !fromNotes {
			warn("lastmod-uniform", "", "every page has the same lastmod: in CI, restore file times from git (fetch-depth: 0) or set `updated` in notes")
		}
	}

	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Path != diags[j].Path {
			return diags[i].Path < diags[j].Path
		}
		return diags[i].Code < diags[j].Code
	})
	return diags
}

func sortedGroups(m map[string][]string) [][]string {
	out := [][]string{}
	for _, group := range m {
		if len(group) > 1 {
			sort.Strings(group)
			out = append(out, group)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func others(group []string, self string) []string {
	out := []string{}
	for _, p := range group {
		if p != self {
			out = append(out, p)
		}
	}
	return out
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func isLocalHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}

// CountSEODiagnostics returns the number of warnings (info findings do not count).
func CountSEODiagnostics(diags []SEODiagnostic) int {
	n := 0
	for _, d := range diags {
		if d.Severity == "warning" {
			n++
		}
	}
	return n
}
