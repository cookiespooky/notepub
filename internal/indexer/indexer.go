package indexer

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/localutil"
	"github.com/cookiespooky/notepub/internal/mediautil"
	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/s3util"
	"github.com/cookiespooky/notepub/internal/urlutil"
	"github.com/cookiespooky/notepub/internal/wikilink"
)

const (
	resolveFileName  = "resolve.json"
	sitemapIndexName = "sitemap-index.xml"
	robotsFileName   = "robots.txt"
)

func Run(ctx context.Context, cfg config.Config) error {
	artifactsDir := cfg.Paths.ArtifactsDir
	snapshotPath := cfg.Paths.SnapshotFile
	snapshotDir := filepath.Dir(snapshotPath)
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return err
	}

	resolvePath := filepath.Join(artifactsDir, resolveFileName)
	lockPath := filepath.Join(snapshotDir, "index.lock")

	lockFile, err := acquireLock(lockPath)
	if err != nil {
		return err
	}
	defer releaseLock(lockFile, lockPath)

	oldIndex, _ := loadResolve(resolvePath)
	oldSnapshot, _ := loadSnapshot(snapshotPath)

	if oldIndex.Routes == nil {
		oldIndex.Routes = map[string]models.RouteEntry{}
	}
	if oldIndex.Meta == nil {
		oldIndex.Meta = map[string]models.MetaEntry{}
	}

	rulesCfg, err := rules.Load(cfg.RulesPath)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}
	if err := ValidateRules(rulesCfg); err != nil {
		return err
	}

	var (
		objects  []s3util.Object
		s3client *s3.Client
	)
	switch cfg.Content.Source {
	case "local":
		objects, err = localutil.ListMarkdown(cfg.Content.LocalDir, cfg.S3.Prefix)
		if err != nil {
			return fmt.Errorf("list local: %w", err)
		}
	case "s3":
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
		objects, err = s3util.ListObjects(ctx, s3client, cfg.S3.Bucket, cfg.S3.Prefix)
		if err != nil {
			return fmt.Errorf("list s3: %w", err)
		}
	default:
		return fmt.Errorf("unsupported content source: %s", cfg.Content.Source)
	}

	current := map[string]s3util.Object{}
	for _, obj := range objects {
		if !strings.HasSuffix(strings.ToLower(obj.Key), ".md") {
			continue
		}
		current[obj.Key] = obj
	}

	oldKeyToPath := map[string]string{}
	for p, rt := range oldIndex.Routes {
		if rt.S3Key != "" {
			oldKeyToPath[rt.S3Key] = p
		}
	}

	keys := make([]string, 0, len(current))
	for k := range current {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	newSnapshot := map[string]models.SnapshotEntry{}
	newIndex := models.ResolveIndex{
		Routes:      map[string]models.RouteEntry{},
		Meta:        map[string]models.MetaEntry{},
		Links:       map[string]map[string][]string{},
		LinkTargets: map[string]map[string][]string{},
		Media:       map[string][]string{},
	}
	usedPaths := map[string]bool{}
	usedSlugs := map[string]bool{}
	typeCounts := map[string]int{}
	errors := []string{}

	for _, key := range keys {
		obj := current[key]
		lm := ""
		if obj.LastModified != nil {
			lm = obj.LastModified.UTC().Format(time.RFC3339)
		}
		newSnapshot[key] = models.SnapshotEntry{ETag: obj.ETag, LastModified: lm}

		old, ok := oldSnapshot[key]
		if ok && old.ETag == obj.ETag {
			if p := oldKeyToPath[key]; p != "" {
				meta := oldIndex.Meta[p]
				route := oldIndex.Routes[p]
				if meta.FM == nil {
					ok = false
				}
				if ok && (oldIndex.LinkTargets == nil || oldIndex.LinkTargets[p] == nil) {
					ok = false
				} else {
					if err := validateExisting(p, meta, rulesCfg, usedPaths, usedSlugs, typeCounts); err != nil {
						errors = append(errors, fmt.Sprintf("%s: %s", key, err))
						continue
					}
					route.LastModified = lm
					newIndex.Meta[p] = meta
					newIndex.Routes[p] = route
					if oldIndex.LinkTargets != nil {
						if linkSet, ok := oldIndex.LinkTargets[p]; ok {
							newIndex.LinkTargets[p] = linkSet
						}
					}
					if oldIndex.Media != nil {
						if media, ok := oldIndex.Media[p]; ok && len(media) > 0 {
							newIndex.Media[p] = media
						}
					}
					continue
				}
			}
		}

		var body []byte
		switch cfg.Content.Source {
		case "local":
			body, err = localutil.FetchObject(cfg.Content.LocalDir, key)
		case "s3":
			body, err = s3util.FetchObject(ctx, s3client, cfg.S3.Bucket, key)
		}
		if err != nil {
			return fmt.Errorf("fetch %s: %w", key, err)
		}
		metaMap, content, err := parseFrontmatter(body)
		if err != nil {
			return fmt.Errorf("parse frontmatter %s: %w", key, err)
		}
		applyFMDefaults(metaMap, rulesCfg.Fields.Defaults)

		core, err := buildCore(metaMap, rulesCfg)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", key, err))
			continue
		}
		typeDef, ok := rulesCfg.Types[core.Type]
		if !ok {
			if isErrorAction(rulesCfg.Validation.UnknownType) {
				errors = append(errors, fmt.Sprintf("%s: unknown type %q", key, core.Type))
			} else {
				log.Printf("unknown type %q (skipped): %s", core.Type, key)
			}
			continue
		}
		if typeDef.Template == "" && rulesCfg.Validation.MissingTemplate.Action == "error" {
			errors = append(errors, fmt.Sprintf("%s: missing template for type %q", key, core.Type))
			continue
		}
		if strings.TrimSpace(typeDef.Permalink) == "" {
			errors = append(errors, fmt.Sprintf("%s: missing permalink for type %q", key, core.Type))
			continue
		}
		pathVal, err := buildPermalink(typeDef.Permalink, core.Slug, rulesCfg)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", key, err))
			continue
		}
		if usedPaths[pathVal] {
			if isErrorAction(rulesCfg.Validation.DuplicateRoute) {
				errors = append(errors, fmt.Sprintf("%s: duplicate route %q", key, pathVal))
				continue
			}
			log.Printf("duplicate route %q (first wins): %s", pathVal, key)
			continue
		}
		if core.Slug != "" {
			slugKey := strings.ToLower(core.Slug)
			if usedSlugs[slugKey] {
				if isErrorAction(rulesCfg.Validation.UniqueSlug) {
					errors = append(errors, fmt.Sprintf("%s: duplicate slug %q", key, core.Slug))
					continue
				}
				log.Printf("duplicate slug %q (first wins): %s", core.Slug, key)
				continue
			}
			usedSlugs[slugKey] = true
		}
		usedPaths[pathVal] = true
		typeCounts[core.Type]++

		metaEntry := buildMetaEntry(metaMap, core, content, cfg.Site, cfg.OGTypeByType, pathVal, key, cfg.S3.Prefix)
		routeEntry := buildRouteEntry(metaMap, metaEntry, key, obj.ETag, lm, pathVal)
		mediaKeys := extractMediaKeysFromContent(string(content), key, cfg.S3.Prefix)
		if len(mediaKeys) > 0 {
			newIndex.Media[pathVal] = mediaKeys
		}

		newIndex.Meta[pathVal] = metaEntry
		newIndex.Routes[pathVal] = routeEntry
		newIndex.LinkTargets[pathVal] = extractRawLinkTargets(metaMap, content, rulesCfg)
	}

	if len(errors) > 0 {
		for _, msg := range errors {
			log.Printf("index validation: %s", msg)
		}
		return fmt.Errorf("index validation failed (%d errors)", len(errors))
	}

	if err := validateTypeCounts(typeCounts, rulesCfg.Validation.SinglePageOfType); err != nil {
		return err
	}

	links, err := resolveLinks(newIndex, rulesCfg, cfg.S3.Prefix)
	if err != nil {
		return err
	}
	newIndex.Links = links
	newIndex.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	if err := writeAtomicJSON(resolvePath, newIndex); err != nil {
		return fmt.Errorf("write resolve: %w", err)
	}
	if err := writeAtomicJSON(snapshotPath, newSnapshot); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := writeSitemaps(artifactsDir, cfg.Site.BaseURL, newIndex, rulesCfg); err != nil {
		return fmt.Errorf("write sitemap: %w", err)
	}
	if err := writeRobots(artifactsDir, cfg.Site.BaseURL, cfg.Robots); err != nil {
		return fmt.Errorf("write robots: %w", err)
	}
	if err := writeSearchIndex(artifactsDir, newIndex, rulesCfg); err != nil {
		return fmt.Errorf("write search: %w", err)
	}
	if err := materializeCollections(artifactsDir, newIndex, rulesCfg); err != nil {
		return fmt.Errorf("materialize collections: %w", err)
	}

	return nil
}

func loadSnapshot(path string) (map[string]models.SnapshotEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]models.SnapshotEntry{}, err
	}
	var snap map[string]models.SnapshotEntry
	if err := json.Unmarshal(data, &snap); err != nil {
		return map[string]models.SnapshotEntry{}, err
	}
	return snap, nil
}

func loadResolve(path string) (models.ResolveIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.ResolveIndex{}, err
	}
	var idx models.ResolveIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return models.ResolveIndex{}, err
	}
	return idx, nil
}

func writeAtomicJSON(path string, payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return fsyncDir(dir)
}

type coreFields struct {
	Type        string
	Slug        string
	Title       string
	Description string
}

func buildCore(meta map[string]interface{}, cfg rules.Rules) (coreFields, error) {
	core := coreFields{
		Type:        stringFromMeta(meta, "type"),
		Slug:        stringFromMeta(meta, "slug"),
		Title:       stringFromMeta(meta, "title"),
		Description: stringFromMeta(meta, "description"),
	}
	for _, field := range cfg.Fields.Required {
		switch field {
		case "slug":
			continue
		case "type":
			if core.Type == "" {
				return core, fmt.Errorf("missing required field type")
			}
		case "title":
			if core.Title == "" {
				return core, fmt.Errorf("missing required field title")
			}
		default:
			if stringFromMeta(meta, field) == "" {
				return core, fmt.Errorf("missing required field %s", field)
			}
		}
	}
	if core.Type == "" {
		return core, fmt.Errorf("missing required field type")
	}
	return core, nil
}

func buildPermalink(template, slugVal string, cfg rules.Rules) (string, error) {
	if template == "" {
		return "", fmt.Errorf("missing permalink template")
	}
	if strings.Contains(template, "{{ slug }}") || strings.Contains(template, "{{slug}}") {
		if strings.TrimSpace(slugVal) == "" {
			if cfg.Validation.PermalinkRequiresSlug.Action == "error" {
				return "", fmt.Errorf("slug required by permalink")
			}
		}
		template = strings.ReplaceAll(template, "{{ slug }}", slugVal)
		template = strings.ReplaceAll(template, "{{slug}}", slugVal)
	}
	pathVal := strings.TrimSpace(template)
	if pathVal == "" {
		return "", fmt.Errorf("empty permalink")
	}
	if !strings.HasPrefix(pathVal, "/") {
		pathVal = "/" + pathVal
	}
	if pathVal != "/" {
		pathVal = strings.TrimRight(pathVal, "/")
	}
	return pathVal, nil
}

func filenameBase(key string) string {
	base := filepath.Base(key)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func buildMetaEntry(meta map[string]interface{}, core coreFields, content []byte, site config.SiteConfig, ogTypeByType map[string]string, pathVal, s3Key, prefix string) models.MetaEntry {
	canonical := stringFromMeta(meta, "canonical")
	if canonical == "" {
		canonical = buildAbsoluteURL(site.BaseURL, pathVal)
	}
	robots := stringFromMeta(meta, "robots")
	noindex := boolFromMeta(meta, "noindex")
	if robots == "" {
		if noindex {
			robots = "noindex, follow"
		} else {
			robots = "index, follow"
		}
	}

	jsonld := jsonFromMeta(meta, "jsonld")
	og := mapFromMeta(meta, "opengraph")
	if len(og) == 0 {
		og = mapFromMeta(meta, "og")
	}
	if og == nil {
		og = map[string]string{}
	}

	ogTitle := firstNonEmpty(
		stringFromMeta(meta, "og_title"),
		stringFromMeta(meta, "title"),
		site.Title,
	)
	ogDescription := firstNonEmpty(
		stringFromMeta(meta, "og_description"),
		stringFromMeta(meta, "description"),
		excerptFromContent(content, 180),
		site.Description,
	)
	ogURL := canonical
	ogType := firstNonEmpty(
		stringFromMeta(meta, "og_type"),
		mapLookup(ogTypeByType, core.Type),
		"website",
	)

	imageURL := ""
	imageAlt := ""
	fmImage := stringFromMeta(meta, "og_image")
	if fmImage != "" {
		imageURL = resolveImageURLFromHref(fmImage, s3Key, prefix, site.MediaBaseURL, site.BaseURL)
	} else if len(content) > 0 {
		imageURL, imageAlt = resolveImageURLFromContentWithAlt(string(content), s3Key, prefix, site.MediaBaseURL, site.BaseURL)
	}
	if imageURL == "" && site.DefaultOGImage != "" {
		imageURL = resolveImageURLFromHref(site.DefaultOGImage, s3Key, prefix, site.MediaBaseURL, site.BaseURL)
	}

	setDefaultOG(og, "title", ogTitle)
	setDefaultOG(og, "description", ogDescription)
	setDefaultOG(og, "url", ogURL)
	setDefaultOG(og, "type", ogType)
	if imageURL != "" {
		setDefaultOG(og, "image", imageURL)
		if imageAlt != "" {
			setDefaultOG(og, "image:alt", imageAlt)
		}
	}

	return models.MetaEntry{
		Type:        core.Type,
		Slug:        core.Slug,
		Title:       core.Title,
		Description: core.Description,
		Canonical:   canonical,
		Robots:      robots,
		OpenGraph:   og,
		JSONLD:      jsonld,
		Image:       imageURL,
		FM:          meta,
	}
}

func buildRouteEntry(meta map[string]interface{}, metaEntry models.MetaEntry, s3Key, etag, lastModified, pathVal string) models.RouteEntry {
	redirectTo := stringFromMeta(meta, "redirect_to")
	status := 200
	if redirectTo != "" {
		status = 301
	}
	noindex := boolFromMeta(meta, "noindex")
	metaHash := metaHashValue(metaEntry)
	return models.RouteEntry{
		S3Key:        s3Key,
		ETag:         etag,
		LastModified: lastModified,
		RedirectTo:   redirectTo,
		NoIndex:      noindex,
		Status:       status,
		RouteETag:    buildRouteETag(pathVal, status, redirectTo, s3Key, etag, lastModified, metaHash),
	}
}

func parseFrontmatter(body []byte) (map[string]interface{}, []byte, error) {
	content := body
	meta := map[string]interface{}{}
	trimmed := bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	normalized := bytes.ReplaceAll(trimmed, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	start := bytes.TrimLeft(normalized, " \t\r\n")
	if bytes.HasPrefix(start, []byte("---")) {
		parts := bytes.SplitN(start, []byte("\n---"), 2)
		if len(parts) == 2 {
			header := bytes.TrimPrefix(parts[0], []byte("---"))
			if err := yaml.Unmarshal(header, &meta); err != nil {
				return nil, nil, fmt.Errorf("frontmatter yaml: %w", err)
			}
			content = parts[1]
		}
	}
	return meta, content, nil
}

func applyFMDefaults(meta map[string]interface{}, defaults map[string]interface{}) {
	for key, val := range defaults {
		if _, ok := meta[key]; ok {
			continue
		}
		meta[key] = val
	}
}

func validateExisting(pathVal string, meta models.MetaEntry, cfg rules.Rules, usedPaths, usedSlugs map[string]bool, typeCounts map[string]int) error {
	if meta.Type == "" {
		return fmt.Errorf("missing type for %s", pathVal)
	}
	typeDef, ok := cfg.Types[meta.Type]
	if !ok {
		if isErrorAction(cfg.Validation.UnknownType) {
			return fmt.Errorf("unknown type %q", meta.Type)
		}
		log.Printf("unknown type %q (skipped): %s", meta.Type, pathVal)
		return fmt.Errorf("unknown type %q", meta.Type)
	}
	if typeDef.Template == "" && cfg.Validation.MissingTemplate.Action == "error" {
		return fmt.Errorf("missing template for type %q", meta.Type)
	}
	if strings.TrimSpace(typeDef.Permalink) == "" {
		return fmt.Errorf("missing permalink for type %q", meta.Type)
	}
	if strings.Contains(typeDef.Permalink, "{{ slug }}") && strings.TrimSpace(meta.Slug) == "" {
		if cfg.Validation.PermalinkRequiresSlug.Action == "error" {
			return fmt.Errorf("slug required by permalink")
		}
	}
	if usedPaths[pathVal] {
		if isErrorAction(cfg.Validation.DuplicateRoute) {
			return fmt.Errorf("duplicate route %q", pathVal)
		}
		log.Printf("duplicate route %q (first wins)", pathVal)
		return fmt.Errorf("duplicate route %q", pathVal)
	}
	usedPaths[pathVal] = true
	if meta.Slug != "" {
		slugKey := strings.ToLower(meta.Slug)
		if usedSlugs[slugKey] {
			if isErrorAction(cfg.Validation.UniqueSlug) {
				return fmt.Errorf("duplicate slug %q", meta.Slug)
			}
			log.Printf("duplicate slug %q (first wins)", meta.Slug)
			return fmt.Errorf("duplicate slug %q", meta.Slug)
		}
		usedSlugs[slugKey] = true
	}
	typeCounts[meta.Type]++
	return nil
}

func validateTypeCounts(counts map[string]int, rules map[string]int) error {
	for typeName, expected := range rules {
		if counts[typeName] != expected {
			return fmt.Errorf("type %q must appear %d time(s), got %d", typeName, expected, counts[typeName])
		}
	}
	return nil
}

func ValidateRules(cfg rules.Rules) error {
	if cfg.Validation.MaterializeRequiresLimit || cfg.Validation.MaterializeGroupByRequiresItemLimit {
		for name, col := range cfg.Collections {
			if !col.Materialize {
				continue
			}
			if cfg.Validation.MaterializeRequiresLimit && col.Limit == 0 {
				return fmt.Errorf("collection %q materialize requires limit", name)
			}
			if cfg.Validation.MaterializeGroupByRequiresItemLimit && col.GroupBy.By != "" && col.GroupBy.ItemLimit == 0 {
				return fmt.Errorf("collection %q materialize requires group_by.item_limit", name)
			}
		}
	}
	return nil
}

func ValidateResolveLinks(idx models.ResolveIndex, cfg rules.Rules, prefix string) error {
	_, err := resolveLinks(idx, cfg, prefix)
	return err
}

func isErrorAction(rule rules.ActionRule) bool {
	return strings.ToLower(strings.TrimSpace(rule.Action)) == "error"
}

type resolverIndex struct {
	byPath          map[string]string
	byPathLower     map[string]string
	byFilename      map[string][]string
	byFilenameLower map[string][]string
	bySlug          map[string]string
	bySlugLower     map[string]string
	typeByPath      map[string]string
	byWiki          map[string]string
}

func extractRawLinkTargets(meta map[string]interface{}, content []byte, cfg rules.Rules) map[string][]string {
	out := map[string][]string{}
	for _, rule := range cfg.Links {
		switch rule.Kind {
		case "field":
			values := extractFieldValues(meta[rule.Field])
			for _, raw := range values {
				target := parseLinkValue(raw, rule.ValueSyntax)
				if target == "" {
					continue
				}
				out[rule.Name] = append(out[rule.Name], target)
			}
		case "wikilinks":
			for _, target := range extractWikiTargets(content) {
				out[rule.Name] = append(out[rule.Name], target)
			}
		}
	}
	return out
}

func extractFieldValues(val interface{}) []string {
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func parseLinkValue(raw string, syntax string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if syntax == "" {
		syntax = "plain"
	}
	if syntax == "auto" && looksLikeWikiLink(raw) {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "[["), "]]")
		return strings.TrimSpace(raw)
	}
	if syntax == "wikilink" {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "[["), "]]")
		return strings.TrimSpace(raw)
	}
	return raw
}

func looksLikeWikiLink(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, "[[") && strings.HasSuffix(raw, "]]")
}

func extractWikiTargets(content []byte) []string {
	text := normalizeLineEndings(string(content))
	matches := wikiLinkRe.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		raw := strings.TrimSpace(match)
		raw = strings.TrimPrefix(raw, "[[")
		raw = strings.TrimSuffix(raw, "]]")
		raw = strings.TrimSpace(raw)
		if raw != "" {
			out = append(out, raw)
		}
	}
	return out
}

func normalizeTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "[[") && strings.HasSuffix(raw, "]]") {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "[["), "]]")
		raw = strings.TrimSpace(raw)
	}
	if parts := strings.SplitN(raw, "|", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	if parts := strings.SplitN(raw, "#", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	if parts := strings.SplitN(raw, "^", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	raw = strings.TrimPrefix(raw, "./")
	raw = strings.TrimPrefix(raw, "/")
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".md")
	raw = strings.TrimSuffix(raw, ".markdown")
	return strings.TrimSpace(raw)
}

func resolveLinks(idx models.ResolveIndex, cfg rules.Rules, prefix string) (map[string]map[string][]string, error) {
	out := map[string]map[string][]string{}
	errors := []string{}
	resolver, err := buildResolverIndex(idx, prefix)
	if err != nil {
		return out, err
	}
	for pathVal, meta := range idx.Meta {
		rawTargets := idx.LinkTargets[pathVal]
		if rawTargets == nil {
			continue
		}
		for _, rule := range cfg.Links {
			if !typeAllowed(meta.Type, rule.FromTypes) {
				continue
			}
			targets := rawTargets[rule.Name]
			if len(targets) == 0 {
				continue
			}
			for _, target := range targets {
				resolved, _, err := ResolveLink(target, rule.ResolveBy, rule.Resolve, resolver)
				if err != nil {
					if shouldErrorOnResolve(err, rule.Resolve) {
						errors = append(errors, fmt.Sprintf("%s: %s", pathVal, err.Error()))
					} else {
						handleResolveError(err, rule.Resolve, target, pathVal, rule.Name)
					}
					continue
				}
				if resolved == "" {
					continue
				}
				targetMeta, ok := idx.Meta[resolved]
				if !ok {
					continue
				}
				if !typeAllowed(targetMeta.Type, rule.ToTypes) {
					continue
				}
				if out[pathVal] == nil {
					out[pathVal] = map[string][]string{}
				}
				out[pathVal][rule.Name] = append(out[pathVal][rule.Name], resolved)
			}
		}
	}
	if len(errors) > 0 {
		for _, msg := range errors {
			log.Printf("link resolve error: %s", msg)
		}
		return out, fmt.Errorf("link resolve failed (%d errors)", len(errors))
	}
	return out, nil
}

func buildResolverIndex(idx models.ResolveIndex, prefix string) (resolverIndex, error) {
	res := resolverIndex{
		byPath:          map[string]string{},
		byPathLower:     map[string]string{},
		byFilename:      map[string][]string{},
		byFilenameLower: map[string][]string{},
		bySlug:          map[string]string{},
		bySlugLower:     map[string]string{},
		typeByPath:      map[string]string{},
		byWiki:          map[string]string{},
	}
	wikiErrors := []string{}
	for pathVal, route := range idx.Routes {
		meta, ok := idx.Meta[pathVal]
		if !ok {
			continue
		}
		res.typeByPath[pathVal] = meta.Type
		if meta.Slug != "" {
			addResolveKey(res.bySlug, res.bySlugLower, meta.Slug, pathVal)
		}
		if route.S3Key != "" {
			rel := normalizePathKey(route.S3Key, prefix)
			if rel != "" {
				addResolveKey(res.byPath, res.byPathLower, rel, pathVal)
			}
			name := filenameBase(route.S3Key)
			if name != "" {
				addResolveListKey(res.byFilename, res.byFilenameLower, name, pathVal)
				if err := addWikiKey(res.byWiki, name, pathVal); err != nil {
					wikiErrors = append(wikiErrors, err.Error())
				}
			}
		}
		for _, alias := range extractAliases(meta.FM) {
			if err := addWikiKey(res.byWiki, alias, pathVal); err != nil {
				wikiErrors = append(wikiErrors, err.Error())
			}
		}
		if meta.Title != "" {
			if err := addWikiKey(res.byWiki, meta.Title, pathVal); err != nil {
				wikiErrors = append(wikiErrors, err.Error())
			}
		}
	}
	if len(wikiErrors) > 0 {
		for _, msg := range wikiErrors {
			log.Printf("wikimap collision: %s", msg)
		}
		return res, fmt.Errorf("wikimap collisions (%d)", len(wikiErrors))
	}
	return res, nil
}

func addResolveKey(rawMap, lowerMap map[string]string, key, pathVal string) {
	if key == "" {
		return
	}
	if _, ok := rawMap[key]; !ok {
		rawMap[key] = pathVal
	}
	lowerKey := strings.ToLower(key)
	if _, ok := lowerMap[lowerKey]; !ok {
		lowerMap[lowerKey] = pathVal
	}
}

func addResolveListKey(rawMap, lowerMap map[string][]string, key, pathVal string) {
	if key == "" {
		return
	}
	rawMap[key] = append(rawMap[key], pathVal)
	lowerKey := strings.ToLower(key)
	lowerMap[lowerKey] = append(lowerMap[lowerKey], pathVal)
}

func addWikiKey(m map[string]string, key, pathVal string) error {
	norm := normalizeWikiKey(key)
	if norm == "" {
		return nil
	}
	if existing, ok := m[norm]; ok && existing != pathVal {
		return fmt.Errorf("%q -> %s (existing %s)", key, pathVal, existing)
	}
	m[norm] = pathVal
	return nil
}

func extractAliases(meta map[string]interface{}) []string {
	if meta == nil {
		return nil
	}
	val, ok := meta["aliases"]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeWikiKey(val string) string {
	return wikilink.NormalizeKey(val)
}

func splitWikiTarget(raw string) (string, string) {
	return wikilink.SplitTarget(raw)
}

func normalizePathKey(s3Key, prefix string) string {
	key := strings.TrimPrefix(s3Key, prefix)
	key = strings.TrimPrefix(key, "/")
	key = strings.TrimSuffix(key, ".md")
	key = strings.TrimSuffix(key, ".markdown")
	return strings.TrimSpace(key)
}

// ResolveLink resolves a target using the requested resolveBy mode and rule order.
// It returns the resolved path and any trailing anchor ("#...") from the target.
func ResolveLink(target string, resolveBy string, rule rules.ResolveRule, res resolverIndex) (string, string, error) {
	baseTarget, tail := splitWikiTarget(target)
	if strings.EqualFold(strings.TrimSpace(resolveBy), "wikimap") {
		if key := normalizeWikiKey(baseTarget); key != "" {
			if resolved, ok := res.byWiki[key]; ok {
				return resolved, tail, nil
			}
		}
		resolved, err := resolveTarget(baseTarget, rule, res)
		return resolved, tail, err
	}
	resolved, err := resolveTarget(baseTarget, rule, res)
	return resolved, tail, err
}

func resolveTarget(target string, rule rules.ResolveRule, res resolverIndex) (string, error) {
	target = normalizeTarget(target)
	if target == "" {
		return "", fmt.Errorf("empty target")
	}
	caseInsensitive := strings.EqualFold(rule.Case, "insensitive")
	key := target
	if caseInsensitive {
		key = strings.ToLower(target)
	}
	order := rule.Order
	if len(order) == 0 {
		order = []string{"path", "filename", "slug"}
	}
	for _, step := range order {
		switch step {
		case "path":
			if caseInsensitive {
				if v, ok := res.byPathLower[key]; ok {
					return v, nil
				}
			} else if v, ok := res.byPath[key]; ok {
				return v, nil
			}
		case "filename":
			if strings.Contains(key, "/") {
				continue
			}
			var matches []string
			if caseInsensitive {
				matches = res.byFilenameLower[key]
			} else {
				matches = res.byFilename[key]
			}
			if len(matches) == 1 {
				return matches[0], nil
			}
			if len(matches) > 1 {
				return "", fmt.Errorf("ambiguous filename %q", target)
			}
		case "slug":
			if caseInsensitive {
				if v, ok := res.bySlugLower[key]; ok {
					return v, nil
				}
			} else if v, ok := res.bySlug[key]; ok {
				return v, nil
			}
		}
	}
	return "", fmt.Errorf("missing target %q", target)
}

func handleResolveError(err error, rule rules.ResolveRule, target, fromPath, linkName string) {
	msg := err.Error()
	if strings.Contains(msg, "ambiguous") && rule.Ambiguity == "error" {
		log.Printf("link resolve error: %s (from %s link %s)", msg, fromPath, linkName)
		return
	}
	if strings.Contains(msg, "missing") && rule.Missing == "warn_skip" {
		log.Printf("link resolve missing: %s (from %s link %s)", target, fromPath, linkName)
		return
	}
}

func shouldErrorOnResolve(err error, rule rules.ResolveRule) bool {
	msg := err.Error()
	if strings.Contains(msg, "ambiguous") && rule.Ambiguity == "error" {
		return true
	}
	if strings.Contains(msg, "missing") && rule.Missing == "error" {
		return true
	}
	return false
}

func typeAllowed(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, t := range allowed {
		if t == value {
			return true
		}
	}
	return false
}

func resolveImageURLFromContent(markdown, s3Key, prefix, mediaBase, baseURL string) string {
	url, _ := resolveImageURLFromContentWithAlt(markdown, s3Key, prefix, mediaBase, baseURL)
	return url
}

func resolveImageURLFromContentWithAlt(markdown, s3Key, prefix, mediaBase, baseURL string) (string, string) {
	href, alt := extractFirstImageWithAlt(markdown)
	if href == "" {
		return "", ""
	}
	url := resolveImageURLFromHref(href, s3Key, prefix, mediaBase, baseURL)
	return url, alt
}

func extractFirstImageWithAlt(markdown string) (string, string) {
	markdown = normalizeLineEndings(markdown)
	if m := embedImageRe.FindStringSubmatch(markdown); len(m) > 1 {
		target, alt := parseEmbedTarget(m[1])
		return target, alt
	}
	if m := mdImageRe.FindStringSubmatch(markdown); len(m) > 2 {
		return strings.TrimSpace(m[2]), strings.TrimSpace(m[1])
	}
	return "", ""
}

func parseEmbedTarget(inner string) (string, string) {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return "", ""
	}
	parts := strings.SplitN(inner, "|", 2)
	target := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return target, ""
	}
	candidate := strings.TrimSpace(parts[1])
	if candidate == "" || sizeRe.MatchString(candidate) {
		return target, ""
	}
	return target, candidate
}

func resolveImageURLFromHref(href, s3Key, prefix, mediaBase, baseURL string) string {
	return mediautil.ResolveMediaAbsolute(href, s3Key, prefix, mediaBase, baseURL)
}

func extractMediaKeysFromContent(markdown, baseKey, prefix string) []string {
	markdown = normalizeLineEndings(markdown)
	keys := map[string]struct{}{}
	addKey := func(key string) {
		if key == "" {
			return
		}
		key = strings.TrimPrefix(key, "/")
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		keys[key] = struct{}{}
	}
	baseKey = strings.TrimPrefix(baseKey, prefix)
	baseKey = strings.TrimPrefix(baseKey, "/")
	baseDir := path.Dir(baseKey)
	if baseDir == "." {
		baseDir = ""
	}

	for _, match := range embedImageRe.FindAllStringSubmatch(markdown, -1) {
		if len(match) < 2 {
			continue
		}
		target, _ := parseEmbedTarget(match[1])
		if key := resolveMediaKey(target, baseDir, prefix); key != "" {
			addKey(key)
		}
	}
	for _, match := range mdImageRe.FindAllStringSubmatch(markdown, -1) {
		if len(match) < 3 {
			continue
		}
		target := strings.TrimSpace(match[2])
		if key := resolveMediaKey(target, baseDir, prefix); key != "" {
			addKey(key)
		}
	}

	if len(keys) == 0 {
		return nil
	}
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func resolveMediaKey(href, baseDir, prefix string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if isExternalURL(href) {
		return ""
	}
	if strings.HasPrefix(href, "/media/") {
		key := strings.TrimPrefix(href, "/media/")
		key = strings.TrimPrefix(key, "/")
		if key == "" {
			return ""
		}
		return key
	}
	if strings.HasPrefix(href, "/") {
		return ""
	}
	if prefix != "" && strings.HasPrefix(href, prefix) {
		key := strings.TrimPrefix(href, prefix)
		return strings.TrimPrefix(key, "/")
	}
	key := href
	if baseDir != "" {
		key = path.Join(baseDir, key)
	}
	return strings.TrimPrefix(key, "/")
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func normalizeLineEndings(markdown string) string {
	return strings.ReplaceAll(markdown, "\r\n", "\n")
}

func excerptFromContent(content []byte, maxLen int) string {
	if len(content) == 0 || maxLen <= 0 {
		return ""
	}
	text := normalizeLineEndings(string(content))
	text = codeFenceRe.ReplaceAllString(text, " ")
	text = inlineCodeRe.ReplaceAllString(text, " ")
	text = embedImageRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := embedImageRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return " "
		}
		target, alt := parseEmbedTarget(parts[1])
		if alt != "" {
			return alt
		}
		return strings.TrimSpace(target)
	})
	text = mdImageRe.ReplaceAllString(text, "$1")
	text = mdLinkRe.ReplaceAllString(text, "$1")
	text = wikiLinkRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(match, "[["), "]]")
		parts := strings.SplitN(inner, "|", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(parts[0])
	})
	text = htmlTagRe.ReplaceAllString(text, " ")
	text = markdownSymbolRe.ReplaceAllString(text, " ")
	text = spaceRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	return truncateAtWord(text, maxLen)
}

func truncateAtWord(text string, maxLen int) string {
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	trunc := strings.TrimSpace(string(runes[:maxLen]))
	lastSpace := strings.LastIndexAny(trunc, " \t\n")
	if lastSpace > maxLen/2 {
		trunc = strings.TrimSpace(trunc[:lastSpace])
	}
	return trunc
}

func isExternalURL(href string) bool {
	lower := strings.ToLower(href)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "//")
}

var (
	embedImageRe     = regexp.MustCompile(`!\[\[([^\]]+)\]\]`)
	mdImageRe        = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	wikiLinkRe       = regexp.MustCompile(`\[\[[^\]]+\]\]`)
	mdLinkRe         = regexp.MustCompile(`\[(?s:.*?)\]\([^)]+\)`)
	codeFenceRe      = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe     = regexp.MustCompile("`[^`]*`")
	htmlTagRe        = regexp.MustCompile(`</?[^>]+>`)
	markdownSymbolRe = regexp.MustCompile(`(?m)^\s*[#>*\-+]+\s*`)
	spaceRe          = regexp.MustCompile(`\s+`)
	sizeRe           = regexp.MustCompile(`^\d+(x\d+)?$`)
)

func buildRouteETag(routePath string, status int, redirectTo, s3Key, etag, lastModified, metaHash string) string {
	h := sha1.New()
	io.WriteString(h, routePath)
	io.WriteString(h, fmt.Sprint(status))
	io.WriteString(h, redirectTo)
	io.WriteString(h, s3Key)
	io.WriteString(h, etag)
	io.WriteString(h, lastModified)
	io.WriteString(h, metaHash)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(h.Sum(nil)))
}

func metaHashValue(meta models.MetaEntry) string {
	h := sha1.New()
	io.WriteString(h, meta.Type)
	io.WriteString(h, meta.Slug)
	io.WriteString(h, meta.Title)
	io.WriteString(h, meta.Description)
	io.WriteString(h, meta.Canonical)
	io.WriteString(h, meta.Robots)
	io.WriteString(h, fmt.Sprint(meta.OpenGraph))
	if len(meta.JSONLD) > 0 {
		_, _ = h.Write(meta.JSONLD)
	}
	io.WriteString(h, meta.Image)
	if meta.FM != nil {
		if data, err := json.Marshal(meta.FM); err == nil {
			_, _ = h.Write(data)
		}
	}
	if meta.Category != nil {
		io.WriteString(h, meta.Category.Slug)
		io.WriteString(h, meta.Category.Title)
		io.WriteString(h, meta.Category.Path)
		io.WriteString(h, meta.Category.Description)
		io.WriteString(h, meta.Category.SourceS3Key)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func stringFromMeta(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	if val, ok := meta[key]; ok {
		switch v := val.(type) {
		case string:
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func mapLookup(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[key])
}

func setDefaultOG(og map[string]string, key, val string) {
	if og == nil || val == "" {
		return
	}
	if _, ok := og[key]; ok {
		return
	}
	og[key] = val
}

func boolFromMeta(meta map[string]interface{}, key string) bool {
	if meta == nil {
		return false
	}
	if val, ok := meta[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case int:
			return v != 0
		case int64:
			return v != 0
		case float64:
			return v != 0
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true", "1", "yes", "y":
				return true
			case "false", "0", "no", "n":
				return false
			default:
				return false
			}
		}
	}
	return false
}

func mapFromMeta(meta map[string]interface{}, key string) map[string]string {
	if meta == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	val, ok := meta[key]
	if !ok {
		return out
	}
	switch v := val.(type) {
	case map[string]interface{}:
		for k, item := range v {
			out[k] = fmt.Sprint(item)
		}
	case map[string]string:
		for k, item := range v {
			out[k] = item
		}
	}
	return out
}

func jsonFromMeta(meta map[string]interface{}, key string) json.RawMessage {
	if meta == nil {
		return nil
	}
	val, ok := meta[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case string:
		trim := strings.TrimSpace(v)
		if trim == "" {
			return nil
		}
		if json.Valid([]byte(trim)) {
			return json.RawMessage(trim)
		}
		b, _ := json.Marshal(v)
		return json.RawMessage(b)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return json.RawMessage(b)
	}
}

func buildAbsoluteURL(baseURL, p string) string {
	return urlutil.JoinBaseURL(baseURL, p)
}

func writeSitemaps(artifactsDir, baseURL string, idx models.ResolveIndex, cfg rules.Rules) error {
	if err := cleanupSitemapChunks(artifactsDir); err != nil {
		return err
	}
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
	chunkSize := 50000
	chunks := [][]urlEntry{}
	for i := 0; i < len(urls); i += chunkSize {
		end := i + chunkSize
		if end > len(urls) {
			end = len(urls)
		}
		chunks = append(chunks, urls[i:end])
	}
	for idx, chunk := range chunks {
		urlset := struct {
			XMLName xml.Name   `xml:"urlset"`
			Xmlns   string     `xml:"xmlns,attr"`
			URLs    []urlEntry `xml:"url"`
		}{
			Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
			URLs:  chunk,
		}
		buf, err := xml.Marshal(urlset)
		if err != nil {
			return err
		}
		xmlBody := []byte(xml.Header + string(buf))
		name := fmt.Sprintf("sitemap-%04d.xml", idx+1)
		if err := os.WriteFile(filepath.Join(artifactsDir, name), xmlBody, 0o644); err != nil {
			return err
		}
	}
	index := struct {
		XMLName  xml.Name `xml:"sitemapindex"`
		Xmlns    string   `xml:"xmlns,attr"`
		Sitemaps []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod,omitempty"`
		} `xml:"sitemap"`
	}{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}
	now := time.Now().UTC().Format("2006-01-02")
	for idx := range chunks {
		loc := buildAbsoluteURL(baseURL, fmt.Sprintf("/sitemap-%04d.xml", idx+1))
		index.Sitemaps = append(index.Sitemaps, struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod,omitempty"`
		}{Loc: loc, LastMod: now})
	}
	buf, err := xml.Marshal(index)
	if err != nil {
		return err
	}
	xmlBody := []byte(xml.Header + string(buf))
	return os.WriteFile(filepath.Join(artifactsDir, sitemapIndexName), xmlBody, 0o644)
}

func cleanupSitemapChunks(artifactsDir string) error {
	matches, err := filepath.Glob(filepath.Join(artifactsDir, "sitemap-*.xml"))
	if err != nil {
		return err
	}
	for _, p := range matches {
		if filepath.Base(p) == sitemapIndexName {
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func writeRobots(artifactsDir, baseURL string, cfg config.RobotsConfig) error {
	lines := []string{
		"User-agent: *",
	}
	if len(cfg.Disallow) == 0 {
		lines = append(lines, "Allow: /")
	} else {
		for _, rule := range cfg.Disallow {
			if strings.TrimSpace(rule) == "" {
				continue
			}
			lines = append(lines, "Disallow: "+rule)
		}
	}
	lines = append(lines, "Sitemap: "+buildAbsoluteURL(baseURL, "/"+sitemapIndexName))
	if extra := strings.TrimSpace(cfg.Extra); extra != "" {
		lines = append(lines, "")
		lines = append(lines, extra)
	}
	body := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(artifactsDir, robotsFileName), []byte(body), 0o644)
}

type searchIndex struct {
	GeneratedAt string       `json:"generated_at"`
	Items       []searchItem `json:"items"`
}

type searchItem struct {
	Title     string  `json:"title,omitempty"`
	Path      string  `json:"path"`
	Snippet   string  `json:"snippet,omitempty"`
	Type      string  `json:"type,omitempty"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
	Score     float64 `json:"score"`
}

func writeSearchIndex(artifactsDir string, idx models.ResolveIndex, cfg rules.Rules) error {
	items := make([]searchItem, 0, len(idx.Meta))
	for pathVal, meta := range idx.Meta {
		route, ok := idx.Routes[pathVal]
		if !ok || route.Status != 200 || route.NoIndex {
			continue
		}
		if len(cfg.Search.IncludeTypes) > 0 && !typeAllowed(meta.Type, cfg.Search.IncludeTypes) {
			continue
		}
		if cfg.Search.ExcludeDrafts && boolFromMeta(meta.FM, "draft") {
			continue
		}
		docType := meta.Type
		if docType == "" {
			docType = "page"
		}
		items = append(items, searchItem{
			Title:     strings.TrimSpace(meta.Title),
			Path:      pathVal,
			Snippet:   strings.TrimSpace(meta.Description),
			Type:      docType,
			UpdatedAt: route.LastModified,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
	})
	payload := searchIndex{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Items:       items,
	}
	return writeAtomicJSON(filepath.Join(artifactsDir, "search.json"), payload)
}

func acquireLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := tryLockFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("indexer lock busy: %w", err)
	}
	return f, nil
}

func releaseLock(f *os.File, path string) {
	if f == nil {
		return
	}
	_ = unlockFile(f)
	_ = f.Close()
	_ = os.Remove(path)
}

func fsyncDir(dir string) error {
	df, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer df.Close()
	return df.Sync()
}
