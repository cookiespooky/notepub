package serve

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/urlutil"
)

// RenderLLMS builds llms.txt (llmstxt.org) from the index: the site's title
// and description, then the indexed pages grouped by content type, each line a
// Markdown link with the page's description. It is made from the notes on
// every build, so it cannot go stale the way a hand-written file does. Its
// bytes depend only on content and config.
func RenderLLMS(idx models.ResolveIndex, cfg config.Config, rulesCfg rules.Rules) ([]byte, bool) {
	if !cfg.LLMS.Enabled {
		return nil, false
	}
	byType := map[string][]string{}
	for p, route := range idx.Routes {
		if route.Status != http.StatusOK || route.NoIndex || route.PageNum > 0 {
			continue
		}
		meta, ok := idx.Meta[p]
		if !ok || boolFromMeta(meta.FM, "draft") || boolFromMeta(meta.FM, "noindex") {
			continue
		}
		if td, ok := rulesCfg.Types[meta.Type]; ok && !td.IncludeIn.Sitemap {
			continue
		}
		byType[meta.Type] = append(byType[meta.Type], p)
	}

	sections := cfg.LLMS.Sections
	if len(sections) == 0 {
		names := make([]string, 0, len(byType))
		for t := range byType {
			names = append(names, t)
		}
		sort.Strings(names)
		for _, t := range names {
			sections = append(sections, config.LLMSSection{Type: t})
		}
	}

	var buf bytes.Buffer
	title := firstNonEmptyString(cfg.Site.Title, cfg.Site.BaseURL)
	fmt.Fprintf(&buf, "# %s\n", oneLine(title))
	if d := oneLine(cfg.Site.Description); d != "" {
		fmt.Fprintf(&buf, "\n> %s\n", d)
	}
	if intro := strings.TrimSpace(cfg.LLMS.Intro); intro != "" {
		fmt.Fprintf(&buf, "\n%s\n", intro)
	}
	for _, sec := range sections {
		paths := byType[sec.Type]
		if len(paths) == 0 {
			continue
		}
		sort.Slice(paths, func(i, j int) bool {
			ti, tj := strings.ToLower(idx.Meta[paths[i]].Title), strings.ToLower(idx.Meta[paths[j]].Title)
			if ti != tj {
				return ti < tj
			}
			return paths[i] < paths[j]
		})
		if cfg.LLMS.Limit > 0 && len(paths) > cfg.LLMS.Limit {
			paths = paths[:cfg.LLMS.Limit]
		}
		fmt.Fprintf(&buf, "\n## %s\n\n", oneLine(firstNonEmptyString(sec.Title, sec.Type)))
		for _, p := range paths {
			meta := idx.Meta[p]
			link := meta.Canonical
			if link == "" {
				link = urlutil.JoinBaseURL(cfg.Site.BaseURL, urlutil.PublicPath(p))
			}
			name := oneLine(firstNonEmptyString(meta.Title, meta.SEOTitle, p))
			name = strings.NewReplacer("[", "(", "]", ")").Replace(name)
			if d := oneLine(meta.Description); d != "" {
				fmt.Fprintf(&buf, "- [%s](%s): %s\n", name, link, d)
			} else {
				fmt.Fprintf(&buf, "- [%s](%s)\n", name, link)
			}
		}
	}
	return buf.Bytes(), true
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
