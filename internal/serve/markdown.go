package serve

import (
	"regexp"
	"strings"

	"github.com/gosimple/slug"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/cookiespooky/notepub/internal/mediautil"
)

var (
	embedRe = regexp.MustCompile(`!\[\[([^\]]+)\]\]`)
	imgRe   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	wikiRe  = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	sizeRe  = regexp.MustCompile(`^\d+(x\d+)?$`)
	fmRe    = regexp.MustCompile(`(?s)^\s*---\s*\n.*?\n---\s*\n`)
)

func normalizeMarkdownImages(markdown, baseKey, prefix, mediaBase string) string {
	markdown = normalizeLineEndings(markdown)
	markdown = stripFrontmatter(markdown)

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
		resolved := mediautil.ResolveMediaLink(pathPart, baseKey, prefix, mediaBase)
		return "![" + alt + "](" + resolved + ")"
	})

	markdown = imgRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := imgRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		alt := parts[1]
		href := strings.TrimSpace(parts[2])
		resolved := mediautil.ResolveMediaLink(href, baseKey, prefix, mediaBase)
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
		resolved := mediautil.ResolveMediaLink(pathPart, baseKey, prefix, mediaBase)
		return "![" + alt + "](" + resolved + ")"
	})

	markdown = imgRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := imgRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		alt := parts[1]
		href := strings.TrimSpace(parts[2])
		resolved := mediautil.ResolveMediaLink(href, baseKey, prefix, mediaBase)
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

func isExternal(href string) bool {
	return mediautil.IsExternal(href)
}

func escapePath(p string) string {
	return mediautil.EscapePath(p)
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
