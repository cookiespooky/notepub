package serve

import (
	"strings"
	"testing"
)

func renderForTest(t *testing.T, markdown string, wiki map[string]string) string {
	t.Helper()
	md := newMarkdownRenderer()
	normalized := normalizeMarkdownImages(markdown, "notes/a.md", "", "")
	normalized = normalizeMarkdownLinks(normalized, wiki)
	var b strings.Builder
	if err := md.Convert([]byte(normalized), &b); err != nil {
		t.Fatalf("render error: %v", err)
	}
	return postprocessRenderedHTML(b.String())
}

func TestMarkdownSupportsInlineObsidianSyntax(t *testing.T) {
	html := renderForTest(t, "==mark== H~2~O x^2^ $E=mc^2$", nil)
	if !strings.Contains(html, "<mark>mark</mark>") {
		t.Fatalf("highlight not rendered: %s", html)
	}
	if !strings.Contains(html, "<sub>2</sub>") {
		t.Fatalf("subscript not rendered: %s", html)
	}
	if !strings.Contains(html, "<sup>2</sup>") {
		t.Fatalf("superscript not rendered: %s", html)
	}
	if !strings.Contains(html, "class=\"math-inline\"") {
		t.Fatalf("inline math wrapper not rendered: %s", html)
	}
}

func TestMarkdownCalloutAndEmbedRender(t *testing.T) {
	wiki := map[string]string{"note": "/note"}
	md := strings.Join([]string{
		"> [!note] Read me",
		"> Body line",
		"",
		"![[Note]]",
	}, "\n")
	html := renderForTest(t, md, wiki)
	if !strings.Contains(html, "class=\"callout callout-note\"") {
		t.Fatalf("callout not rendered: %s", html)
	}
	if !strings.Contains(html, "class=\"obsidian-embed\"") {
		t.Fatalf("embed not rendered: %s", html)
	}
	if !strings.Contains(html, "href=\"/note\"") {
		t.Fatalf("embed link not resolved: %s", html)
	}
}

func TestMarkdownVideoEmbedRender(t *testing.T) {
	html := renderForTest(t, "![[demo.mp4]]", nil)
	if !strings.Contains(html, `<video controls preload="metadata" src="/media/notes/demo.mp4"></video>`) {
		t.Fatalf("video embed not rendered: %s", html)
	}
}

func TestMarkdownFootnoteAndRawHTML(t *testing.T) {
	md := "Footnote ref[^1].\n\n[^1]: Value\n\n<span data-x=\"1\">ok</span>"
	html := renderForTest(t, md, nil)
	if !strings.Contains(html, "footnote") {
		t.Fatalf("footnote not rendered: %s", html)
	}
	if !strings.Contains(html, "<span data-x=\"1\">ok</span>") {
		t.Fatalf("raw html not preserved: %s", html)
	}
}
