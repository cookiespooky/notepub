package wikilink

import (
	"strings"

	"golang.org/x/text/cases"
)

var fold = cases.Fold()

// NormalizeKey applies Obsidian-like normalization: trim, unicode case-fold,
// and collapse internal whitespace.
func NormalizeKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = fold.String(value)
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// SplitTarget parses a wiki-like target and returns the normalized base target
// (without label or heading) and the trailing anchor (e.g. "#Heading").
func SplitTarget(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if strings.HasPrefix(raw, "[[") && strings.HasSuffix(raw, "]]") {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "[["), "]]")
		raw = strings.TrimSpace(raw)
	}
	if raw == "" {
		return "", ""
	}
	if parts := strings.SplitN(raw, "|", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	tail := ""
	if idx := strings.Index(raw, "#"); idx >= 0 {
		tail = raw[idx:]
		raw = strings.TrimSpace(raw[:idx])
	}
	raw = strings.TrimPrefix(raw, "./")
	raw = strings.TrimPrefix(raw, "/")
	raw = strings.TrimSuffix(raw, ".md")
	raw = strings.TrimSuffix(raw, ".markdown")
	raw = strings.TrimSpace(raw)
	return raw, tail
}
