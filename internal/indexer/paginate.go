package indexer

import (
	"fmt"
	"strings"

	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/paginate"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/urlutil"
)

// synthesizePaginatedRoutes adds routes for pages 2..N of every type that
// declares `paginate`. Page 1 keeps the type's own permalink and is left alone.
//
// It has to run after validateTypeCounts — a paginated singleton would trip
// single_page_of_type otherwise — and before resolve.json is written, so the
// sitemap and the static builder see the new routes without knowing about them.
//
// The synthesised entries deliberately carry no identity: their Slug is cleared
// so they stay out of the slug index and the wiki map, and PageNum keeps them
// out of collections and the search index.
func synthesizePaginatedRoutes(idx *models.ResolveIndex, cfg rules.Rules, baseURL string, usedPaths map[string]bool) error {
	slugIndex := buildSlugIndex(*idx)
	backrefs := buildBackrefs(*idx)

	for typeName, typeDef := range cfg.Types {
		rule := typeDef.Paginate
		if rule == nil {
			continue
		}
		if err := validatePaginateRule(typeName, *rule, cfg); err != nil {
			return err
		}
		colRule := cfg.Collections[rule.Collection]
		items, ok := collectionItems(*idx, colRule, cfg, slugIndex, backrefs)
		if !ok {
			return fmt.Errorf("type %q: paginate collection %q has unsupported kind %q", typeName, rule.Collection, colRule.Kind)
		}
		total := paginate.PageCount(len(items), rule.PerPage)
		if total < 2 {
			continue
		}

		for basePath, meta := range idx.Meta {
			if meta.Type != typeName || idx.Routes[basePath].PageNum > 0 {
				continue
			}
			baseRoute := idx.Routes[basePath]
			if baseRoute.Status != 200 {
				continue
			}
			for n := 2; n <= total; n++ {
				pagePath := paginate.Path(expandSlug(rule.Path, meta.Slug), n)
				if pagePath == "" {
					return fmt.Errorf("type %q: empty paginate path for page %d", typeName, n)
				}
				pagePath = strings.TrimSuffix(pagePath, "/")
				if pagePath == "" {
					pagePath = "/"
				}
				if usedPaths[pagePath] {
					return fmt.Errorf("type %q: paginated route %q collides with an existing page", typeName, pagePath)
				}
				usedPaths[pagePath] = true

				pageRoute := baseRoute
				pageRoute.PageNum = n
				pageRoute.PaginateOf = basePath

				pageMeta := meta
				// Cleared on purpose: two routes answering to one slug make
				// wikilinks and collection lookups depend on map order.
				pageMeta.Slug = ""
				pageMeta.Canonical = buildAbsoluteURL(baseURL, urlutil.PublicPath(pagePath))

				idx.Routes[pagePath] = pageRoute
				idx.Meta[pagePath] = pageMeta
			}
		}
	}
	return nil
}

func validatePaginateRule(typeName string, rule rules.PaginateRule, cfg rules.Rules) error {
	if rule.Collection == "" {
		return fmt.Errorf("type %q: paginate.collection is required", typeName)
	}
	if _, ok := cfg.Collections[rule.Collection]; !ok {
		return fmt.Errorf("type %q: paginate.collection %q is not declared", typeName, rule.Collection)
	}
	if rule.PerPage <= 0 {
		return fmt.Errorf("type %q: paginate.per_page must be positive", typeName)
	}
	if rule.Path == "" {
		return fmt.Errorf("type %q: paginate.path is required", typeName)
	}
	if !strings.Contains(rule.Path, "{{ n }}") && !strings.Contains(rule.Path, "{{n}}") {
		return fmt.Errorf("type %q: paginate.path must contain {{ n }}", typeName)
	}
	return nil
}

// expandSlug fills the optional {{ slug }} placeholder, which a paginated type
// needs only when it has more than one page of that type.
func expandSlug(tmpl, slug string) string {
	out := strings.ReplaceAll(tmpl, "{{ slug }}", slug)
	return strings.ReplaceAll(out, "{{slug}}", slug)
}
