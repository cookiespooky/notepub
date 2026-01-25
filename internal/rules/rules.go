package rules

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Rules struct {
	Version     int                       `yaml:"version"`
	Fields      FieldContract             `yaml:"fields"`
	FMSchema    map[string]string         `yaml:"fm_schema"`
	Types       map[string]TypeDef        `yaml:"types"`
	Links       []LinkRule                `yaml:"links"`
	Collections map[string]CollectionRule `yaml:"collections"`
	Sitemap     SitemapRule               `yaml:"sitemap"`
	Search      SearchRule                `yaml:"search"`
	Artifacts   ArtifactsRule             `yaml:"artifacts"`
	Validation  ValidationRule            `yaml:"validation"`
}

type FieldContract struct {
	Required []string               `yaml:"required"`
	Optional []string               `yaml:"optional"`
	Defaults map[string]interface{} `yaml:"defaults"`
}

type TypeDef struct {
	Template  string        `yaml:"template"`
	Permalink string        `yaml:"permalink"`
	IncludeIn IncludeInRule `yaml:"include_in"`
}

type IncludeInRule struct {
	Sitemap bool `yaml:"sitemap"`
	Search  bool `yaml:"search"`
}

type LinkRule struct {
	Name        string      `yaml:"name"`
	Kind        string      `yaml:"kind"`
	Field       string      `yaml:"field,omitempty"`
	FromTypes   []string    `yaml:"from_types"`
	ToTypes     []string    `yaml:"to_types"`
	To          string      `yaml:"to"`
	Inverse     string      `yaml:"inverse,omitempty"`
	ValueSyntax string      `yaml:"value_syntax,omitempty"`
	Resolve     ResolveRule `yaml:"resolve"`
	ResolveBy   string      `yaml:"resolve_by,omitempty"`
}

type ResolveRule struct {
	Order     []string `yaml:"order"`
	Ambiguity string   `yaml:"ambiguity,omitempty"`
	Missing   string   `yaml:"missing,omitempty"`
	Case      string   `yaml:"case,omitempty"`
}

type CollectionRule struct {
	Kind        string      `yaml:"kind"`
	Materialize bool        `yaml:"materialize"`
	Link        string      `yaml:"link,omitempty"`
	FromSlug    string      `yaml:"from_slug,omitempty"`
	ToSlug      string      `yaml:"to_slug,omitempty"`
	Where       WhereRule   `yaml:"where"`
	Sort        SortRule    `yaml:"sort"`
	Limit       int         `yaml:"limit"`
	GroupBy     GroupByRule `yaml:"group_by"`
}

type WhereRule struct {
	All []map[string]interface{} `yaml:"all"`
}

type SortRule struct {
	By        string `yaml:"by"`
	Dir       string `yaml:"dir"`
	NullsLast bool   `yaml:"nulls_last"`
}

type GroupByRule struct {
	By        string   `yaml:"by"`
	Multi     bool     `yaml:"multi"`
	GroupSort SortRule `yaml:"group_sort"`
	ItemSort  SortRule `yaml:"item_sort"`
	ItemLimit int      `yaml:"item_limit"`
}

type SitemapRule struct {
	IncludeTypes  []string `yaml:"include_types"`
	ExcludeDrafts bool     `yaml:"exclude_drafts"`
}

type SearchRule struct {
	IncludeTypes  []string          `yaml:"include_types"`
	ExcludeDrafts bool              `yaml:"exclude_drafts"`
	FieldsBoost   SearchFieldsBoost `yaml:"fields_boost"`
	Preview       SearchPreview     `yaml:"preview"`
}

type SearchFieldsBoost struct {
	Title       float64            `yaml:"title"`
	Description float64            `yaml:"description"`
	Body        float64            `yaml:"body"`
	FM          map[string]float64 `yaml:"fm"`
}

type SearchPreview struct {
	From   string `yaml:"from"`
	MaxLen int    `yaml:"max_len"`
}

type ArtifactsRule struct {
	Collections CollectionsArtifactsRule `yaml:"collections"`
}

type CollectionsArtifactsRule struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

type ValidationRule struct {
	SinglePageOfType                    map[string]int `yaml:"single_page_of_type"`
	DuplicateRoute                      ActionRule     `yaml:"duplicate_route"`
	UnknownType                         ActionRule     `yaml:"unknown_type"`
	UniqueSlug                          ActionRule     `yaml:"unique_slug"`
	PermalinkRequiresSlug               ActionRule     `yaml:"permalink_requires_slug"`
	MissingTemplate                     ActionRule     `yaml:"missing_template"`
	MaterializeRequiresLimit            bool           `yaml:"materialize_requires_limit"`
	MaterializeGroupByRequiresItemLimit bool           `yaml:"materialize_group_by_requires_item_limit"`
}

type ActionRule struct {
	Action string `yaml:"action"`
}

func Load(path string) (Rules, error) {
	if path == "" {
		return Rules{Version: 1}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Rules{}, err
	}
	var out Rules
	if err := yaml.Unmarshal(data, &out); err != nil {
		return Rules{}, err
	}
	if out.Version == 0 {
		out.Version = 1
	}
	if out.Fields.Defaults == nil {
		out.Fields.Defaults = map[string]interface{}{}
	}
	if out.FMSchema == nil {
		out.FMSchema = map[string]string{}
	}
	if out.Types == nil {
		out.Types = map[string]TypeDef{}
	}
	if out.Collections == nil {
		out.Collections = map[string]CollectionRule{}
	}
	return out, nil
}
