package indexer

import (
	"testing"

	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

func TestResolveLinkWikiMap(t *testing.T) {
	idx := models.ResolveIndex{
		Routes: map[string]models.RouteEntry{
			"/note": {S3Key: "notes/My Note.md", Status: 200},
		},
		Meta: map[string]models.MetaEntry{
			"/note": {
				Slug:  "my-note",
				Title: "Different Title",
				FM: map[string]interface{}{
					"aliases": []string{"Alt Name"},
				},
			},
		},
	}
	resolver, err := buildResolverIndex(idx, "")
	if err != nil {
		t.Fatalf("buildResolverIndex: %v", err)
	}
	rule := rules.ResolveRule{}

	tests := []struct {
		name      string
		target    string
		wantPath  string
		wantTail  string
		resolveBy string
	}{
		{"basename", "My Note", "/note", "", "wikimap"},
		{"alias", "Alt Name", "/note", "", "wikimap"},
		{"label", "[[My Note|Label]]", "/note", "", "wikimap"},
		{"heading", "[[My Note#Heading]]", "/note", "#Heading", "wikimap"},
		{"block", "[[My Note#^block]]", "/note", "#^block", "wikimap"},
		{"fallback-slug", "my-note", "/note", "", "wikimap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, tail, err := ResolveLink(tt.target, tt.resolveBy, rule, resolver)
			if err != nil {
				t.Fatalf("ResolveLink error: %v", err)
			}
			if got != tt.wantPath || tail != tt.wantTail {
				t.Fatalf("ResolveLink(%q) = (%q, %q), want (%q, %q)", tt.target, got, tail, tt.wantPath, tt.wantTail)
			}
		})
	}
}

func TestResolverIndexWikiMapCollision(t *testing.T) {
	idx := models.ResolveIndex{
		Routes: map[string]models.RouteEntry{
			"/a": {S3Key: "a/Note.md", Status: 200},
			"/b": {S3Key: "b/Note.md", Status: 200},
		},
		Meta: map[string]models.MetaEntry{
			"/a": {Title: "A"},
			"/b": {Title: "B"},
		},
	}
	if _, err := buildResolverIndex(idx, ""); err == nil {
		t.Fatalf("expected wikimap collision error")
	}
}
