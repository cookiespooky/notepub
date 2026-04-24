package serve

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	rendererhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/cookiespooky/notepub/internal/linkutil"
	"github.com/cookiespooky/notepub/internal/mdproc"
	"github.com/cookiespooky/notepub/internal/mediautil"
)

var (
	embedRe       = regexp.MustCompile(`!\[\[([^\]]+)\]\]`)
	imgRe         = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	wikiRe        = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	sizeRe        = regexp.MustCompile(`^\d+(x\d+)?$`)
	fmRe          = regexp.MustCompile(`(?s)^\s*---\s*\n.*?\n---\s*\n`)
	imageExtRe    = regexp.MustCompile(`(?i)\.(png|jpe?g|gif|webp|svg|avif|bmp|ico|tiff?|heic|heif)$`)
	videoExtRe    = regexp.MustCompile(`(?i)\.(mp4|webm|ogv|mov|m4v)$`)
	highlightRe   = regexp.MustCompile(`==([^=\n][^=\n]*?)==`)
	inlineMathRe  = regexp.MustCompile(`\$(.+?)\$`)
	calloutHTMLRe = regexp.MustCompile(`(?s)<blockquote>\s*<p>\[!([A-Za-z0-9_-]+)\]([+-])?\s*([^\n<]*)\n?(.*?)</p>\s*</blockquote>`)
	mdLinkPattern = regexp.MustCompile(`^\[([^\]]+)\]\(([^)]+)\)$`)
)

func normalizeMarkdownImages(markdown, baseKey, prefix, mediaBase string) string {
	markdown = mdproc.NormalizeLineEndings(markdown)
	markdown = stripFrontmatter(markdown)

	return mdproc.RewriteOutsideCode(markdown, func(segment string) string {
		segment = embedRe.ReplaceAllStringFunc(segment, func(match string) string {
			parts := embedRe.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			inner := strings.TrimSpace(parts[1])
			if inner == "" {
				return match
			}
			pathPart, alt := splitEmbed(inner)
			pathPart = linkutil.StripWikiAnchor(pathPart)
			if isImageTarget(pathPart) {
				resolved := mediautil.ResolveMediaLink(pathPart, baseKey, prefix, mediaBase)
				return "![" + alt + "](" + resolved + ")"
			}
			if isVideoTarget(pathPart) {
				resolved := mediautil.ResolveMediaLink(pathPart, baseKey, prefix, mediaBase)
				return `<video controls preload="metadata" src="` + html.EscapeString(resolved) + `"></video>`
			}
			return match
		})

		segment = imgRe.ReplaceAllStringFunc(segment, func(match string) string {
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

		return segment
	})
}

func normalizeMarkdownImagesForBuild(markdown, baseKey, prefix, mediaBase string) string {
	return normalizeMarkdownImages(markdown, baseKey, prefix, mediaBase)
}

func normalizeMarkdownLinks(markdown string, wikiMap map[string]string, baseURL string) string {
	markdown = transformBlockMath(markdown)
	if len(wikiMap) == 0 {
		return mdproc.RewriteOutsideCode(markdown, applyObsidianInlineSyntax)
	}
	return mdproc.RewriteOutsideCode(markdown, func(segment string) string {
		segment = replaceWikiLinksAndEmbeds(segment, wikiMap, baseURL)
		return applyObsidianInlineSyntax(segment)
	})
}

func stripFrontmatter(markdown string) string {
	out := fmRe.ReplaceAllString(markdown, "")
	return strings.TrimPrefix(out, "\n")
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

func buildWikiLink(targetRaw string, wikiMap map[string]string, baseURL string) string {
	targetPart, heading, display := linkutil.SplitWikiParts(targetRaw)
	if targetPart == "" {
		return targetRaw
	}
	target := linkutil.NormalizeWikiTarget(targetPart)
	if target == "" {
		return targetRaw
	}
	if display == "" {
		display = target
	}
	pathVal, ok := wikiMap[strings.ToLower(target)]
	if !ok {
		return display
	}
	if heading != "" {
		if strings.HasPrefix(heading, "^") {
			pathVal += "#" + heading
		} else {
			anchor := slug.MakeLang(heading, "en")
			if anchor != "" {
				pathVal = pathVal + "#" + anchor
			}
		}
	}
	return "[" + display + "](" + withBaseURL(pathVal, baseURL) + ")"
}

func replaceWikiLinksAndEmbeds(segment string, wikiMap map[string]string, baseURL string) string {
	if segment == "" {
		return segment
	}
	matches := wikiRe.FindAllStringSubmatchIndex(segment, -1)
	if len(matches) == 0 {
		return segment
	}
	var out strings.Builder
	last := 0
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		start, end := m[0], m[1]
		innerStart, innerEnd := m[2], m[3]
		if start < last {
			continue
		}
		isEmbed := start > 0 && segment[start-1] == '!'
		if isEmbed {
			out.WriteString(segment[last : start-1])
		} else {
			out.WriteString(segment[last:start])
		}

		inner := strings.TrimSpace(segment[innerStart:innerEnd])
		if inner == "" {
			if isEmbed {
				out.WriteString(segment[start:end])
			}
			last = end
			continue
		}

		if isEmbed {
			link := buildWikiLink(inner, wikiMap, baseURL)
			if label, href, ok := parseMarkdownLink(link); ok {
				out.WriteString(`<div class="obsidian-embed"><span class="obsidian-embed-label">Embedded:</span> <a href="` + html.EscapeString(href) + `">` + html.EscapeString(label) + `</a></div>`)
			} else {
				out.WriteString(`<div class="obsidian-embed"><span class="obsidian-embed-label">Embedded:</span> ` + html.EscapeString(link) + `</div>`)
			}
		} else {
			out.WriteString(buildWikiLink(inner, wikiMap, baseURL))
		}
		last = end
	}
	if last < len(segment) {
		out.WriteString(segment[last:])
	}
	return out.String()
}

func parseMarkdownLink(v string) (label string, href string, ok bool) {
	m := mdLinkPattern.FindStringSubmatch(strings.TrimSpace(v))
	if len(m) < 3 {
		return "", "", false
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
}

func withBaseURL(pathVal, baseURL string) string {
	if pathVal == "" || strings.HasPrefix(pathVal, "http://") || strings.HasPrefix(pathVal, "https://") {
		return pathVal
	}
	if !strings.HasPrefix(pathVal, "/") {
		return pathVal
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return pathVal
	}
	return base + pathVal
}

func applyObsidianInlineSyntax(segment string) string {
	if segment == "" {
		return segment
	}
	segment = highlightRe.ReplaceAllString(segment, "<mark>$1</mark>")
	segment = replaceWrapped(segment, '^', "sup")
	segment = replaceWrapped(segment, '~', "sub")
	segment = inlineMathRe.ReplaceAllStringFunc(segment, func(m string) string {
		if len(m) < 3 {
			return m
		}
		inner := m[1 : len(m)-1]
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return m
		}
		return `<span class="math-inline">` + html.EscapeString(inner) + `</span>`
	})
	return segment
}

func replaceWrapped(segment string, wrapper byte, tag string) string {
	if segment == "" {
		return segment
	}
	var out strings.Builder
	i := 0
	for i < len(segment) {
		if segment[i] != wrapper {
			out.WriteByte(segment[i])
			i++
			continue
		}
		if i+1 < len(segment) && segment[i+1] == wrapper {
			out.WriteByte(segment[i])
			i++
			continue
		}
		j := i + 1
		for j < len(segment) && segment[j] != wrapper && segment[j] != '\n' {
			j++
		}
		if j >= len(segment) || segment[j] != wrapper || j == i+1 {
			out.WriteByte(segment[i])
			i++
			continue
		}
		inner := strings.TrimSpace(segment[i+1 : j])
		if inner == "" {
			out.WriteByte(segment[i])
			i++
			continue
		}
		out.WriteString("<" + tag + ">" + inner + "</" + tag + ">")
		i = j + 1
	}
	return out.String()
}

func transformBlockMath(markdown string) string {
	markdown = mdproc.NormalizeLineEndings(markdown)
	lines := strings.SplitAfter(markdown, "\n")
	var out strings.Builder

	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	inMath := false
	var mathBuf strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inMath {
			if trimmed == "$$" {
				out.WriteString("<div class=\"math-block\">" + html.EscapeString(strings.TrimSpace(mathBuf.String())) + "</div>\n")
				inMath = false
				mathBuf.Reset()
				continue
			}
			mathBuf.WriteString(line)
			continue
		}

		if inFence {
			out.WriteString(line)
			if isFenceCloseLine(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}

		if ch, n, ok := parseFenceOpenLine(line); ok {
			inFence = true
			fenceChar = ch
			fenceLen = n
			out.WriteString(line)
			continue
		}

		if trimmed == "$$" {
			inMath = true
			continue
		}
		out.WriteString(line)
	}
	if inMath {
		out.WriteString("$$\n" + mathBuf.String())
	}
	return out.String()
}

func parseFenceOpenLine(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return 0, 0, false
	}
	ch := trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return ch, n, true
}

func isFenceCloseLine(line string, ch byte, min int) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" || trimmed[0] != ch {
		return false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	if n < min {
		return false
	}
	return strings.TrimSpace(trimmed[n:]) == ""
}

func postprocessRenderedHTML(body string) string {
	if body == "" {
		return body
	}
	return calloutHTMLRe.ReplaceAllStringFunc(body, func(match string) string {
		m := calloutHTMLRe.FindStringSubmatch(match)
		if len(m) < 5 {
			return match
		}
		typ := strings.ToLower(strings.TrimSpace(m[1]))
		fold := strings.TrimSpace(m[2])
		title := strings.TrimSpace(m[3])
		content := strings.TrimSpace(m[4])
		if title == "" {
			if typ == "" {
				title = "Note"
			} else {
				title = strings.ToUpper(typ[:1]) + typ[1:]
			}
		}
		if fold == "-" || fold == "+" {
			open := ""
			if fold == "+" {
				open = " open"
			}
			return fmt.Sprintf(`<details class="callout callout-%s"%s><summary class="callout-title">%s</summary><div class="callout-body">%s</div></details>`, typ, open, title, content)
		}
		return fmt.Sprintf(`<div class="callout callout-%s"><div class="callout-title">%s</div><div class="callout-body">%s</div></div>`, typ, title, content)
	})
}

func isImageTarget(target string) bool {
	return imageExtRe.MatchString(cleanMediaTarget(target))
}

func isVideoTarget(target string) bool {
	return videoExtRe.MatchString(cleanMediaTarget(target))
}

func cleanMediaTarget(target string) string {
	lower := strings.ToLower(strings.TrimSpace(target))
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return lower
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
			extension.Footnote,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			rendererhtml.WithUnsafe(),
		),
	)
}
