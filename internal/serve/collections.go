package serve

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

func buildCollections(idx models.ResolveIndex, cfg rules.Rules, currentPath string) map[string]models.CollectionResult {
	if len(cfg.Collections) == 0 {
		return nil
	}
	currentMeta := idx.Meta[currentPath]
	slugIndex := buildSlugIndex(idx)
	backrefs := buildBackrefs(idx)

	out := map[string]models.CollectionResult{}
	for name, rule := range cfg.Collections {
		items := []models.CollectionItem{}
		switch rule.Kind {
		case "filter":
			for pathVal := range idx.Meta {
				items = append(items, buildCollectionItem(idx, pathVal))
			}
		case "forward":
			if idx.Links == nil {
				break
			}
			fromSlug := resolveTemplate(rule.FromSlug, currentMeta.Slug)
			if fromSlug == "" {
				fromSlug = currentMeta.Slug
			}
			fromPath := slugIndex[fromSlug]
			if fromPath == "" {
				break
			}
			for _, target := range idx.Links[fromPath][rule.Link] {
				items = append(items, buildCollectionItem(idx, target))
			}
		case "backrefs":
			if idx.Links == nil {
				break
			}
			toSlug := resolveTemplate(rule.ToSlug, currentMeta.Slug)
			if toSlug == "" {
				toSlug = currentMeta.Slug
			}
			toPath := slugIndex[toSlug]
			if toPath == "" {
				break
			}
			for _, source := range backrefs[rule.Link][toPath] {
				items = append(items, buildCollectionItem(idx, source))
			}
		default:
			continue
		}

		items = filterItems(items, rule.Where, cfg.FMSchema)
		if rule.Sort.By != "" {
			sortItems(items, rule.Sort, cfg.FMSchema)
		}
		if rule.Limit > 0 && len(items) > rule.Limit {
			items = items[:rule.Limit]
		}

		result := models.CollectionResult{}
		if rule.GroupBy.By != "" {
			result.Groups = groupItems(items, rule.GroupBy, cfg.FMSchema)
		} else {
			result.Items = items
		}
		out[name] = result
	}
	return out
}

func buildCollectionItem(idx models.ResolveIndex, pathVal string) models.CollectionItem {
	meta := idx.Meta[pathVal]
	route := idx.Routes[pathVal]
	return models.CollectionItem{
		Path:        pathVal,
		Type:        meta.Type,
		Slug:        meta.Slug,
		Title:       meta.Title,
		Description: meta.Description,
		Canonical:   meta.Canonical,
		Image:       meta.Image,
		UpdatedAt:   route.LastModified,
		NoIndex:     route.NoIndex,
		FM:          meta.FM,
	}
}

func buildSlugIndex(idx models.ResolveIndex) map[string]string {
	out := map[string]string{}
	for pathVal, meta := range idx.Meta {
		if meta.Slug == "" {
			continue
		}
		if _, ok := out[meta.Slug]; !ok {
			out[meta.Slug] = pathVal
		}
	}
	return out
}

func buildBackrefs(idx models.ResolveIndex) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for from, links := range idx.Links {
		for name, targets := range links {
			if out[name] == nil {
				out[name] = map[string][]string{}
			}
			for _, to := range targets {
				out[name][to] = append(out[name][to], from)
			}
		}
	}
	return out
}

func resolveTemplate(val string, slug string) string {
	if val == "" {
		return ""
	}
	return strings.ReplaceAll(val, "{{ page.slug }}", slug)
}

func filterItems(items []models.CollectionItem, where rules.WhereRule, fmSchema map[string]string) []models.CollectionItem {
	if len(where.All) == 0 {
		return items
	}
	out := make([]models.CollectionItem, 0, len(items))
	for _, item := range items {
		if matchWhere(item, where, fmSchema) {
			out = append(out, item)
		}
	}
	return out
}

func matchWhere(item models.CollectionItem, where rules.WhereRule, fmSchema map[string]string) bool {
	for _, clause := range where.All {
		for key, val := range clause {
			switch key {
			case "type_in":
				if !valueInList(item.Type, val) {
					return false
				}
			case "fm_eq":
				m, ok := val.(map[string]interface{})
				if !ok {
					return false
				}
				keyVal, _ := m["key"].(string)
				exp := m["value"]
				if !fmEquals(item.FM, keyVal, exp, fmSchema) {
					return false
				}
			}
		}
	}
	return true
}

func valueInList(value string, list interface{}) bool {
	switch v := list.(type) {
	case []interface{}:
		for _, item := range v {
			if fmt.Sprint(item) == value {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if item == value {
				return true
			}
		}
	}
	return false
}

func fmEquals(fm map[string]interface{}, key string, expected interface{}, schema map[string]string) bool {
	if fm == nil || key == "" {
		return false
	}
	val, ok := fm[key]
	if !ok {
		return false
	}
	typ := schema[key]
	switch typ {
	case "number":
		return toFloat(val) == toFloat(expected)
	case "boolean":
		return toBool(val) == toBool(expected)
	default:
		return strings.TrimSpace(fmt.Sprint(val)) == strings.TrimSpace(fmt.Sprint(expected))
	}
}

func sortItems(items []models.CollectionItem, rule rules.SortRule, schema map[string]string) {
	sort.SliceStable(items, func(i, j int) bool {
		vi, ni := sortValue(items[i], rule.By, schema)
		vj, nj := sortValue(items[j], rule.By, schema)
		if ni && nj {
			return false
		}
		if ni != nj {
			if rule.NullsLast {
				return !ni
			}
			return ni
		}
		less := compareValues(vi, vj)
		if strings.ToLower(rule.Dir) == "desc" {
			return !less
		}
		return less
	})
}

func sortValue(item models.CollectionItem, by string, schema map[string]string) (interface{}, bool) {
	switch by {
	case "title":
		if item.Title == "" {
			return "", true
		}
		return strings.ToLower(item.Title), false
	case "slug":
		if item.Slug == "" {
			return "", true
		}
		return strings.ToLower(item.Slug), false
	case "updated_at", "created_at":
		if item.UpdatedAt == "" {
			return "", true
		}
		return item.UpdatedAt, false
	default:
		if strings.HasPrefix(by, "fm.") {
			key := strings.TrimPrefix(by, "fm.")
			val, ok := item.FM[key]
			if !ok {
				return nil, true
			}
			switch schema[key] {
			case "number":
				return toFloat(val), false
			case "boolean":
				return toBool(val), false
			default:
				return strings.ToLower(fmt.Sprint(val)), false
			}
		}
	}
	return "", true
}

func compareValues(a, b interface{}) bool {
	switch va := a.(type) {
	case float64:
		vb, _ := b.(float64)
		return va < vb
	case bool:
		vb, _ := b.(bool)
		return !va && vb
	default:
		return fmt.Sprint(a) < fmt.Sprint(b)
	}
}

func groupItems(items []models.CollectionItem, rule rules.GroupByRule, schema map[string]string) []models.CollectionGroup {
	groups := map[string][]models.CollectionItem{}
	for _, item := range items {
		keys := groupKeys(item, rule.By, rule.Multi, schema)
		if len(keys) == 0 {
			continue
		}
		for _, key := range keys {
			groups[key] = append(groups[key], item)
		}
	}
	groupKeysList := make([]string, 0, len(groups))
	for key := range groups {
		groupKeysList = append(groupKeysList, key)
	}
	sort.Strings(groupKeysList)
	if strings.ToLower(rule.GroupSort.Dir) == "desc" {
		sort.Sort(sort.Reverse(sort.StringSlice(groupKeysList)))
	}
	out := make([]models.CollectionGroup, 0, len(groupKeysList))
	for _, key := range groupKeysList {
		groupItems := groups[key]
		if rule.ItemSort.By != "" {
			sortItems(groupItems, rule.ItemSort, schema)
		}
		if rule.ItemLimit > 0 && len(groupItems) > rule.ItemLimit {
			groupItems = groupItems[:rule.ItemLimit]
		}
		out = append(out, models.CollectionGroup{Key: key, Items: groupItems})
	}
	return out
}

func groupKeys(item models.CollectionItem, by string, multi bool, schema map[string]string) []string {
	switch by {
	case "type":
		if item.Type == "" {
			return nil
		}
		return []string{item.Type}
	default:
		if strings.HasPrefix(by, "fm.") {
			key := strings.TrimPrefix(by, "fm.")
			val, ok := item.FM[key]
			if !ok {
				return nil
			}
			if multi {
				switch v := val.(type) {
				case []interface{}:
					keys := make([]string, 0, len(v))
					for _, item := range v {
						keys = append(keys, fmt.Sprint(item))
					}
					return keys
				case []string:
					return v
				}
			}
			return []string{fmt.Sprint(val)}
		}
	}
	return nil
}

func toFloat(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		f, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		return f
	}
}

func toBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true
		default:
			return false
		}
	default:
		return fmt.Sprint(val) == "true"
	}
}
