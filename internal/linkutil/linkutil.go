package linkutil

import (
	"regexp"
	"strings"
)

var mdFieldLinkRe = regexp.MustCompile(`^!?\[[^\]]*\]\(([^)]+)\)$`)

func LooksLikeWikiLink(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, "[[") && strings.HasSuffix(raw, "]]")
}

func UnwrapWikiLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if LooksLikeWikiLink(raw) {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "[["), "]]"))
	}
	return raw
}

func SplitWikiParts(raw string) (target string, heading string, alias string) {
	raw = UnwrapWikiLink(raw)
	if raw == "" {
		return "", "", ""
	}
	if parts := strings.SplitN(raw, "|", 2); len(parts) == 2 {
		raw = strings.TrimSpace(parts[0])
		alias = strings.TrimSpace(parts[1])
	}
	if parts := strings.SplitN(raw, "#", 2); len(parts) == 2 {
		raw = strings.TrimSpace(parts[0])
		heading = strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(raw), strings.TrimSpace(heading), strings.TrimSpace(alias)
}

func NormalizeWikiTarget(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimSuffix(name, ".markdown")
	return strings.TrimSpace(name)
}

func NormalizeTargetForResolve(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = UnwrapWikiLink(raw)
	if parts := strings.SplitN(raw, "|", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	if parts := strings.SplitN(raw, "#", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	if parts := strings.SplitN(raw, "^", 2); len(parts) > 0 {
		raw = strings.TrimSpace(parts[0])
	}
	return NormalizeWikiTarget(raw)
}

func StripWikiAnchor(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if idx := strings.Index(target, "#"); idx >= 0 {
		target = strings.TrimSpace(target[:idx])
	}
	return target
}

func ParseMarkdownLinkTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	m := mdFieldLinkRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	target := strings.TrimSpace(m[1])
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") && len(target) > 2 {
		target = strings.TrimSpace(target[1 : len(target)-1])
	}
	target = stripMarkdownLinkTitle(target)
	return strings.TrimSpace(target)
}

func stripMarkdownLinkTitle(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return target
	}
	if strings.Contains(target, " ") {
		parts := strings.Fields(target)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return target
}
