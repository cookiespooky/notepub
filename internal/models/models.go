package models

import "encoding/json"

type SnapshotEntry struct {
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

type ResolveIndex struct {
	GeneratedAt string                         `json:"generated_at"`
	Routes      map[string]RouteEntry          `json:"routes"`
	Meta        map[string]MetaEntry           `json:"meta"`
	Links       map[string]map[string][]string `json:"links,omitempty"`
	LinkTargets map[string]map[string][]string `json:"link_targets,omitempty"`
	Media       map[string][]string            `json:"media,omitempty"`
}

type RouteEntry struct {
	S3Key        string `json:"s3_key"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	RedirectTo   string `json:"redirect_to,omitempty"`
	NoIndex      bool   `json:"noindex,omitempty"`
	Status       int    `json:"status"`
	RouteETag    string `json:"route_etag,omitempty"`
}

type MetaEntry struct {
	Type        string                 `json:"type,omitempty"`
	Slug        string                 `json:"slug,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Canonical   string                 `json:"canonical,omitempty"`
	Robots      string                 `json:"robots,omitempty"`
	OpenGraph   map[string]string      `json:"opengraph,omitempty"`
	JSONLD      json.RawMessage        `json:"jsonld,omitempty"`
	Category    *CategoryModel         `json:"category,omitempty"`
	Image       string                 `json:"image,omitempty"`
	FM          map[string]interface{} `json:"fm,omitempty"`
}

type CategoryModel struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	SourceS3Key string `json:"source_s3_key,omitempty"`
	Image       string `json:"image,omitempty"`
}

type Catalog struct {
	GeneratedAt string            `json:"generated_at"`
	Categories  []CatalogCategory `json:"categories"`
}

type CatalogCategory struct {
	CategoryModel
	Count int           `json:"count"`
	Items []CatalogItem `json:"items"`
}

type CatalogItem struct {
	Path        string `json:"path"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Canonical   string `json:"canonical,omitempty"`
	NoIndex     bool   `json:"noindex"`
	Image       string `json:"image,omitempty"`
}

type CollectionResult struct {
	Items  []CollectionItem  `json:"items,omitempty"`
	Groups []CollectionGroup `json:"groups,omitempty"`
}

type CollectionGroup struct {
	Key   string           `json:"key"`
	Items []CollectionItem `json:"items"`
}

type CollectionItem struct {
	Path        string                 `json:"path"`
	Type        string                 `json:"type,omitempty"`
	Slug        string                 `json:"slug,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Canonical   string                 `json:"canonical,omitempty"`
	Image       string                 `json:"image,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
	NoIndex     bool                   `json:"noindex"`
	FM          map[string]interface{} `json:"fm,omitempty"`
}
