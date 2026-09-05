package serve

import (
	"bytes"
	"strings"
	"testing"
)

// Cyrillic headings used to render as id="-" or id="heading" because
// goldmark's own generator drops multi-byte runes.
func TestHeadingIDsTransliterate(t *testing.T) {
	md := newMarkdownRenderer()
	src := []byte("## Что сделано\n\ntext\n\n## Задача\n\ntext\n\n## Что сделано\n")
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{`id="chto-sdelano"`, `id="zadacha"`, `id="chto-sdelano-1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	for _, bad := range []string{`id="-"`, `id="heading"`, `id="--1"`} {
		if strings.Contains(got, bad) {
			t.Errorf("still emitting %s", bad)
		}
	}
}

// A [[page#Heading]] wikilink must resolve to the id the heading actually gets.
func TestWikilinkAnchorMatchesHeadingID(t *testing.T) {
	if got, want := headingAnchor("Что сделано"), "chto-sdelano"; got != want {
		t.Errorf("headingAnchor = %q, want %q", got, want)
	}
}
