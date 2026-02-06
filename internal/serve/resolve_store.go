package serve

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
	"github.com/cookiespooky/notepub/internal/wikilink"
)

type ResolveStore struct {
	path          string
	mu            sync.RWMutex
	mtime         time.Time
	idx           models.ResolveIndex
	wiki          map[string]string
	search        []searchDoc
	media         map[string]struct{}
	rules         rules.Rules
	allowAllMedia bool
}

func NewResolveStore(path string, rulesCfg rules.Rules, allowAllMedia bool) *ResolveStore {
	return &ResolveStore{path: path, rules: rulesCfg, allowAllMedia: allowAllMedia}
}

func (s *ResolveStore) Get() (models.ResolveIndex, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return s.cachedOrError(err)
	}
	s.mu.RLock()
	cachedMtime := s.mtime
	s.mu.RUnlock()
	if info.ModTime().After(cachedMtime) {
		if err := s.reload(info.ModTime()); err != nil {
			log.Printf("resolve reload failed: %v", err)
			return s.cachedOrError(err)
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx, nil
}

func (s *ResolveStore) GetWithWikiMap() (models.ResolveIndex, map[string]string, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return s.cachedWithWikiOrError(err)
	}
	s.mu.RLock()
	cachedMtime := s.mtime
	s.mu.RUnlock()
	if info.ModTime().After(cachedMtime) {
		if err := s.reload(info.ModTime()); err != nil {
			log.Printf("resolve reload failed: %v", err)
			return s.cachedWithWikiOrError(err)
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx, cloneWikiMap(s.wiki), nil
}

func (s *ResolveStore) GetWithWikiMapContext(ctx context.Context) (models.ResolveIndex, map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return s.cachedWithWikiOrError(err)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return s.cachedWithWikiOrError(err)
	}
	s.mu.RLock()
	cachedMtime := s.mtime
	s.mu.RUnlock()
	if info.ModTime().After(cachedMtime) {
		if err := s.reload(info.ModTime()); err != nil {
			log.Printf("resolve reload failed: %v", err)
			return s.cachedWithWikiOrError(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return s.cachedWithWikiOrError(err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx, cloneWikiMap(s.wiki), nil
}

func (s *ResolveStore) Search(query string, limit int, cursor string) ([]SearchItem, string, error) {
	idx, docs, err := s.getSearchDocs()
	if err != nil {
		return nil, "", err
	}
	_ = idx
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return []SearchItem{}, "", nil
	}
	results := make([]scoredItem, 0)
	for _, doc := range docs {
		score := scoreQuery(doc, q, s.rules.Search.FieldsBoost)
		if score <= 0 {
			continue
		}
		item := doc.toItem()
		item.Score = score
		results = append(results, scoredItem{SearchItem: item, score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
		}
		return results[i].score > results[j].score
	})
	offset := decodeCursor(cursor)
	if offset < 0 || offset > len(results) {
		offset = 0
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}
	items := make([]SearchItem, 0, end-offset)
	for i := offset; i < end; i++ {
		items = append(items, results[i].SearchItem)
	}
	nextCursor := ""
	if end < len(results) {
		nextCursor = encodeCursor(end)
	}
	return items, nextCursor, nil
}

func (s *ResolveStore) reload(mtime time.Time) error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var idx models.ResolveIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return err
	}
	s.mu.Lock()
	s.idx = idx
	s.wiki = buildWikiMap(idx)
	s.search = buildSearchIndex(idx, s.rules)
	s.media = buildMediaAllowlist(idx)
	s.mtime = mtime
	s.mu.Unlock()
	return nil
}

func (s *ResolveStore) cachedOrError(err error) (models.ResolveIndex, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mtime.IsZero() {
		return models.ResolveIndex{}, err
	}
	return s.idx, nil
}

func (s *ResolveStore) cachedWithWikiOrError(err error) (models.ResolveIndex, map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mtime.IsZero() {
		return models.ResolveIndex{}, nil, err
	}
	return s.idx, cloneWikiMap(s.wiki), nil
}

func (s *ResolveStore) MediaAllowed(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.allowAllMedia {
		return true
	}
	if s.media == nil {
		return false
	}
	_, ok := s.media[key]
	return ok
}

func (s *ResolveStore) getSearchDocs() (models.ResolveIndex, []searchDoc, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return s.cachedSearchOrError(err)
	}
	s.mu.RLock()
	cachedMtime := s.mtime
	s.mu.RUnlock()
	if info.ModTime().After(cachedMtime) {
		if err := s.reload(info.ModTime()); err != nil {
			log.Printf("resolve reload failed: %v", err)
			return s.cachedSearchOrError(err)
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx, append([]searchDoc{}, s.search...), nil
}

func (s *ResolveStore) cachedSearchOrError(err error) (models.ResolveIndex, []searchDoc, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mtime.IsZero() {
		return models.ResolveIndex{}, nil, err
	}
	return s.idx, append([]searchDoc{}, s.search...), nil
}

func buildWikiMap(idx models.ResolveIndex) map[string]string {
	out := map[string]string{}
	for pathVal, meta := range idx.Meta {
		route, ok := idx.Routes[pathVal]
		if ok && route.S3Key != "" {
			name := filenameBase(route.S3Key)
			addWikiKey(out, name, pathVal)
		}
		for _, alias := range extractAliases(meta.FM) {
			addWikiKey(out, alias, pathVal)
		}
		addWikiKey(out, meta.Title, pathVal)
		addWikiKey(out, meta.Slug, pathVal)
		if ok && route.S3Key != "" {
			if rel := normalizePathKey(route.S3Key); rel != "" {
				addWikiKey(out, rel, pathVal)
			}
		}
	}
	return out
}

type searchDoc struct {
	Path        string
	Title       string
	Description string
	UpdatedAt   string
	Type        string
	lowerTitle  string
	lowerDesc   string
	lowerPath   string
	extras      map[string]string
}

type SearchItem struct {
	Title     string  `json:"title"`
	Path      string  `json:"path"`
	Snippet   string  `json:"snippet,omitempty"`
	Score     float64 `json:"score"`
	Type      string  `json:"type"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
}

type scoredItem struct {
	SearchItem
	score float64
}

func buildSearchIndex(idx models.ResolveIndex, cfg rules.Rules) []searchDoc {
	docs := make([]searchDoc, 0, len(idx.Meta))
	for pathVal, meta := range idx.Meta {
		route, ok := idx.Routes[pathVal]
		if !ok || route.Status != 200 {
			continue
		}
		if route.NoIndex || boolFromMeta(meta.FM, "noindex") {
			continue
		}
		if len(cfg.Search.IncludeTypes) > 0 && !typeAllowed(meta.Type, cfg.Search.IncludeTypes) {
			continue
		}
		if cfg.Search.ExcludeDrafts && boolFromMeta(meta.FM, "draft") {
			continue
		}
		title := strings.TrimSpace(meta.Title)
		desc := strings.TrimSpace(meta.Description)
		updated := route.LastModified
		docType := meta.Type
		if docType == "" {
			docType = "page"
		}
		doc := searchDoc{
			Path:        pathVal,
			Title:       title,
			Description: desc,
			UpdatedAt:   updated,
			Type:        docType,
			lowerTitle:  strings.ToLower(title),
			lowerDesc:   strings.ToLower(desc),
			lowerPath:   strings.ToLower(pathVal),
		}
		doc.extras = extractSearchExtras(meta.FM, cfg.Search.FieldsBoost.FM)
		docs = append(docs, doc)
	}
	return docs
}

func (d searchDoc) toItem() SearchItem {
	return SearchItem{
		Title:     d.Title,
		Path:      d.Path,
		Snippet:   d.Description,
		Type:      d.Type,
		UpdatedAt: d.UpdatedAt,
	}
}

func scoreQuery(doc searchDoc, q string, boost rules.SearchFieldsBoost) float64 {
	score := 0.0
	titleBoost := boost.Title
	descBoost := boost.Description
	bodyBoost := boost.Body
	if titleBoost == 0 {
		titleBoost = 2.0
	}
	if descBoost == 0 {
		descBoost = 1.0
	}
	if bodyBoost == 0 {
		bodyBoost = 0.5
	}
	if strings.Contains(doc.lowerTitle, q) {
		score += titleBoost
	}
	if strings.Contains(doc.lowerDesc, q) {
		score += descBoost
	}
	if strings.Contains(doc.lowerPath, q) {
		score += bodyBoost
	}
	if doc.extras != nil {
		for key, val := range doc.extras {
			if !strings.Contains(val, q) {
				continue
			}
			if b, ok := boost.FM[key]; ok && b > 0 {
				score += b
			} else {
				score += 0.5
			}
		}
	}
	return score
}

func encodeCursor(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	val, err := strconv.Atoi(string(decoded))
	if err != nil || val < 0 {
		return 0
	}
	return val
}

func addWikiKey(m map[string]string, key, pathVal string) {
	norm := normalizeWikiKey(key)
	if norm == "" {
		return
	}
	if existing, ok := m[norm]; ok && existing != pathVal {
		log.Printf("duplicate wikilink key: %s -> %s (existing %s)", key, pathVal, existing)
		return
	}
	m[norm] = pathVal
}

func normalizeWikiKey(val string) string {
	return wikilink.NormalizeKey(val)
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

func filenameBase(key string) string {
	base := path.Base(key)
	base = strings.TrimSuffix(base, path.Ext(base))
	return strings.TrimSpace(base)
}

func normalizePathKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	key = strings.TrimSuffix(key, ".md")
	key = strings.TrimSuffix(key, ".markdown")
	return strings.TrimSpace(key)
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

func extractSearchExtras(meta map[string]interface{}, fmBoost map[string]float64) map[string]string {
	if meta == nil || len(fmBoost) == 0 {
		return nil
	}
	out := map[string]string{}
	for key := range fmBoost {
		if val, ok := meta[key]; ok {
			out[key] = strings.ToLower(strings.TrimSpace(fmt.Sprint(val)))
		}
	}
	return out
}

func cloneWikiMap(src map[string]string) map[string]string {
	if src == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func buildMediaAllowlist(idx models.ResolveIndex) map[string]struct{} {
	if len(idx.Media) == 0 {
		return nil
	}
	out := map[string]struct{}{}
	for _, keys := range idx.Media {
		for _, key := range keys {
			key = strings.TrimSpace(key)
			key = strings.TrimPrefix(key, "/")
			if key == "" {
				continue
			}
			out[key] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
