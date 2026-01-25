package serve

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	embedRe = regexp.MustCompile(`!\[\[([^\]]+)\]\]`)
	imgRe   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	wikiRe  = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	sizeRe  = regexp.MustCompile(`^\d+(x\d+)?$`)
	fmRe    = regexp.MustCompile(`(?s)^\s*---\s*\n.*?\n---\s*\n`)
)

func normalizeMarkdownImages(markdown, baseKey, prefix string) string {
	markdown = normalizeLineEndings(markdown)
	markdown = stripFrontmatter(markdown)
	baseDir := path.Dir(strings.TrimPrefix(baseKey, "/"))
	if baseDir == "." {
		baseDir = ""
	}

	markdown = embedRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := embedRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		inner := strings.TrimSpace(parts[1])
		if inner == "" {
			return match
		}
		pathPart, alt := splitEmbed(inner)
		resolved := resolveMediaPath(pathPart, baseDir, prefix)
		return "![" + alt + "](" + resolved + ")"
	})

	markdown = imgRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := imgRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		alt := parts[1]
		href := strings.TrimSpace(parts[2])
		resolved := resolveMediaPath(href, baseDir, prefix)
		if resolved == href {
			return match
		}
		return "![" + alt + "](" + resolved + ")"
	})

	return markdown
}

func normalizeMarkdownImagesForBuild(markdown, baseKey, prefix, mediaBase string) string {
	markdown = normalizeLineEndings(markdown)
	markdown = stripFrontmatter(markdown)
	baseDir := path.Dir(strings.TrimPrefix(baseKey, "/"))
	if baseDir == "." {
		baseDir = ""
	}

	markdown = embedRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := embedRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		inner := strings.TrimSpace(parts[1])
		if inner == "" {
			return match
		}
		pathPart, alt := splitEmbed(inner)
		resolved := resolveMediaURLForBuild(pathPart, baseDir, prefix, mediaBase)
		return "![" + alt + "](" + resolved + ")"
	})

	markdown = imgRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := imgRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		alt := parts[1]
		href := strings.TrimSpace(parts[2])
		resolved := resolveMediaURLForBuild(href, baseDir, prefix, mediaBase)
		if resolved == href {
			return match
		}
		return "![" + alt + "](" + resolved + ")"
	})

	return markdown
}

func normalizeMarkdownLinks(markdown string, wikiMap map[string]string) string {
	if len(wikiMap) == 0 {
		return markdown
	}
	return wikiRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := wikiRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		targetRaw := strings.TrimSpace(parts[1])
		if targetRaw == "" {
			return match
		}
		targetPart := targetRaw
		display := ""
		if segs := strings.SplitN(targetRaw, "|", 2); len(segs) > 1 {
			targetPart = strings.TrimSpace(segs[0])
			display = strings.TrimSpace(segs[1])
		}
		targetPart = strings.TrimSpace(targetPart)
		if targetPart == "" {
			return match
		}
		target, heading := splitHeading(targetPart)
		target = normalizeWikiTarget(target)
		if target == "" {
			return match
		}
		if display == "" {
			display = target
		}
		pathVal, ok := wikiMap[strings.ToLower(target)]
		if !ok {
			return display
		}
		if heading != "" {
			anchor := slug.MakeLang(heading, "en")
			if anchor != "" {
				pathVal = pathVal + "#" + anchor
			}
		}
		return "[" + display + "](" + pathVal + ")"
	})
}

func stripFrontmatter(markdown string) string {
	out := fmRe.ReplaceAllString(markdown, "")
	return strings.TrimPrefix(out, "\n")
}

func normalizeLineEndings(markdown string) string {
	markdown = strings.TrimPrefix(markdown, "\ufeff")
	return strings.ReplaceAll(markdown, "\r\n", "\n")
}

func splitEmbed(inner string) (string, string) {
	segments := strings.Split(inner, "|")
	if len(segments) == 0 {
		return inner, ""
	}
	pathPart := strings.TrimSpace(segments[0])
	alt := ""
	if len(segments) > 1 {
		candidate := strings.TrimSpace(segments[1])
		if candidate != "" && !sizeRe.MatchString(candidate) {
			alt = candidate
		}
	}
	return pathPart, alt
}

func splitHeading(target string) (string, string) {
	parts := strings.SplitN(target, "#", 2)
	if len(parts) == 1 {
		return target, ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func normalizeWikiTarget(name string) string {
	if name == "" {
		return name
	}
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimSuffix(name, ".markdown")
	return strings.TrimSpace(name)
}

func resolveMediaPath(href, baseDir, prefix string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return href
	}
	if isExternal(href) || strings.HasPrefix(href, "/") {
		return href
	}
	key := href
	if prefix != "" && strings.HasPrefix(key, prefix) {
		// already a full key
	} else if baseDir != "" {
		key = path.Join(baseDir, key)
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return href
	}
	return "/media/" + escapePath(key)
}

func resolveMediaURLForBuild(href, baseDir, prefix, mediaBase string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return href
	}
	if isExternal(href) {
		return href
	}
	mediaBase = strings.TrimRight(mediaBase, "/")
	if strings.HasPrefix(href, "/media/") {
		if mediaBase == "" {
			return href
		}
		key := strings.TrimPrefix(href, "/media/")
		key = strings.TrimPrefix(key, "/")
		if key == "" {
			return href
		}
		return mediaBase + "/" + escapePath(key)
	}
	if strings.HasPrefix(href, "/") {
		return href
	}
	key := href
	if prefix != "" && strings.HasPrefix(key, prefix) {
		key = strings.TrimPrefix(key, prefix)
		key = strings.TrimPrefix(key, "/")
	} else if baseDir != "" {
		key = path.Join(baseDir, key)
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return href
	}
	if mediaBase == "" {
		return "/media/" + escapePath(key)
	}
	return mediaBase + "/" + escapePath(key)
}

func isExternal(href string) bool {
	lower := strings.ToLower(href)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "//")
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func newMarkdownRenderer() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
			extension.Table,
			extension.TaskList,
			extension.Linkify,
		),
	)
}
