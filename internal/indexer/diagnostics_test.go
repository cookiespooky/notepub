package indexer

import (
	"strings"
	"testing"

	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

func testResolver(t *testing.T) resolverIndex {
	t.Helper()
	idx := models.ResolveIndex{
		Routes: map[string]models.RouteEntry{
			"/note": {S3Key: "notes/Note.md", Status: 200},
		},
		Meta: map[string]models.MetaEntry{
			"/note": {Slug: "note", Title: "Note", FM: map[string]interface{}{}},
		},
	}
	res, err := buildResolverIndex(idx, "")
	if err != nil {
		t.Fatalf("buildResolverIndex: %v", err)
	}
	return res
}

func TestDiagnoseMarkdownContent(t *testing.T) {
	res := testResolver(t)
	rule := rules.ResolveRule{Order: []string{"path", "filename", "slug"}, Ambiguity: "error", Missing: "error", Case: "insensitive"}

	md := strings.Join([]string{
		"Good [[Note]]",
		"Missing [[Unknown]]",
		"Missing embed ![[Ghost]]",
		"Image embed ![[cover.png]]",
		"<span>raw</span>",
		"`[[CodeIgnored]]`",
	}, "\n")

	diags := diagnoseMarkdownContent("content/a.md", md, res, rule, "safe")
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d: %#v", len(diags), diags)
	}

	codes := map[string]bool{}
	for _, d := range diags {
		codes[d.Code] = true
	}
	if !codes["NP-MD-WIKI-MISSING"] {
		t.Fatalf("missing NP-MD-WIKI-MISSING: %#v", diags)
	}
	if !codes["NP-MD-EMBED-MISSING"] {
		t.Fatalf("missing NP-MD-EMBED-MISSING: %#v", diags)
	}
	if !codes["NP-MD-HTML-SANITIZED"] {
		t.Fatalf("missing NP-MD-HTML-SANITIZED: %#v", diags)
	}
}

func TestCountDiagnostics(t *testing.T) {
	diags := []MarkdownDiagnostic{
		{Severity: "error"},
		{Severity: "warn"},
		{Severity: "warning"},
	}
	errN, warnN := CountDiagnostics(diags)
	if errN != 1 || warnN != 2 {
		t.Fatalf("CountDiagnostics = (%d, %d), want (1,2)", errN, warnN)
	}
}

func TestDiagnoseMarkdownContentPolicyDeny(t *testing.T) {
	res := testResolver(t)
	rule := rules.ResolveRule{Order: []string{"path", "filename", "slug"}, Ambiguity: "error", Missing: "error", Case: "insensitive"}
	diags := diagnoseMarkdownContent("content/a.md", "<span>x</span>", res, rule, "deny")
	if len(diags) != 1 || diags[0].Code != "NP-MD-RAW-HTML-DENY" || diags[0].Severity != "error" {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
}

func TestDiagnoseMarkdownContentBlockRefUnsupported(t *testing.T) {
	res := testResolver(t)
	rule := rules.ResolveRule{Order: []string{"path", "filename", "slug"}, Ambiguity: "error", Missing: "error", Case: "insensitive"}
	diags := diagnoseMarkdownContent("content/a.md", "Block ref [[Note#^block-1]]", res, rule, "safe")
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for block ref")
	}
	found := false
	for _, d := range diags {
		if d.Code == "NP-OBSIDIAN-UNSUPPORTED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected NP-OBSIDIAN-UNSUPPORTED in %#v", diags)
	}
}
