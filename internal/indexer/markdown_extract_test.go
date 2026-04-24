package indexer

import (
	"strings"
	"testing"

	"github.com/cookiespooky/notepub/internal/rules"
)

func TestExtractFirstImageWithAltSkipsNonImageEmbeds(t *testing.T) {
	md := strings.Join([]string{
		"![[Note#Section]]",
		"![[cover.png|Cover Alt]]",
	}, "\n")

	href, alt := extractFirstImageWithAlt(md)
	if href != "cover.png" || alt != "Cover Alt" {
		t.Fatalf("extractFirstImageWithAlt() = (%q, %q), want (%q, %q)", href, alt, "cover.png", "Cover Alt")
	}
}

func TestExtractFirstImageWithAltIgnoresCode(t *testing.T) {
	md := strings.Join([]string{
		"```md",
		"![[fake.png]]",
		"```",
		"![Real](real.png)",
	}, "\n")

	href, alt := extractFirstImageWithAlt(md)
	if href != "real.png" || alt != "Real" {
		t.Fatalf("extractFirstImageWithAlt() = (%q, %q), want (%q, %q)", href, alt, "real.png", "Real")
	}
}

func TestExtractMediaKeysFromContentIncludesVideoEmbeds(t *testing.T) {
	got := extractMediaKeysFromContent("![[demo.mp4]]\n![Cover](cover.png)", "home.md", "")
	want := []string{"cover.png", "demo.mp4"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("media keys = %#v, want %#v", got, want)
	}
}

func TestExtractWikiTargetsIgnoresCode(t *testing.T) {
	md := strings.Join([]string{
		"Visible [[Real Link]]",
		"Media embed ![[demo.mp4]]",
		"`[[Inline Hidden]]`",
		"```",
		"[[Fence Hidden]]",
		"```",
	}, "\n")

	targets := extractWikiTargets([]byte(md))
	if len(targets) != 1 || targets[0] != "Real Link" {
		t.Fatalf("extractWikiTargets() = %#v, want only Real Link", targets)
	}
}

func TestParseLinkValueMarkdownLink(t *testing.T) {
	cases := []struct {
		raw    string
		syntax string
		want   string
	}{
		{"[A](/path/to/page)", "markdown_link", "/path/to/page"},
		{"![Img](media/pic.png)", "markdown_image", "media/pic.png"},
		{"[A](./local.md \"title\")", "auto", "./local.md"},
		{"[[Wiki|Label]]", "auto", "Wiki|Label"},
	}
	for _, tc := range cases {
		got := parseLinkValue(tc.raw, tc.syntax)
		if got != tc.want {
			t.Fatalf("parseLinkValue(%q, %q) = %q, want %q", tc.raw, tc.syntax, got, tc.want)
		}
	}
}

func TestExtractObsidianTags(t *testing.T) {
	md := strings.Join([]string{
		"# Heading",
		"Text with #docs and #docs/engine tags.",
		"`#inline_code_tag` should be ignored",
		"```",
		"#fenced_tag",
		"```",
	}, "\n")
	tags := extractObsidianTags([]byte(md))
	if len(tags) != 2 || tags[0] != "docs" || tags[1] != "docs/engine" {
		t.Fatalf("extractObsidianTags() = %#v, want [docs docs/engine]", tags)
	}
}

func TestExtractRawLinkTargetsKinds(t *testing.T) {
	meta := map[string]interface{}{}
	content := []byte(strings.Join([]string{
		"Markdown link [Home](/home)",
		"Image embed ![Cover](media/cover.png)",
		"Wiki embed ![[Note#Section|Title]]",
		"Bare URL https://example.com/docs/page.",
	}, "\n"))
	cfg := rules.Rules{
		Links: []rules.LinkRule{
			{Name: "md", Kind: "markdown_links"},
			{Name: "emb", Kind: "embeds"},
			{Name: "auto", Kind: "auto_links"},
		},
	}
	got := extractRawLinkTargets(meta, content, cfg)

	if len(got["md"]) != 1 || got["md"][0] != "/home" {
		t.Fatalf("markdown_links = %#v, want [/home]", got["md"])
	}
	if len(got["emb"]) != 2 || got["emb"][0] != "Note#Section" || got["emb"][1] != "media/cover.png" {
		t.Fatalf("embeds = %#v, want [Note#Section media/cover.png]", got["emb"])
	}
	if len(got["auto"]) != 1 || got["auto"][0] != "https://example.com/docs/page" {
		t.Fatalf("auto_links = %#v, want [https://example.com/docs/page]", got["auto"])
	}
}
