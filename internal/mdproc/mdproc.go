package mdproc

import "strings"

// NormalizeLineEndings normalizes markdown line endings to \n and strips UTF-8 BOM.
func NormalizeLineEndings(markdown string) string {
	markdown = strings.TrimPrefix(markdown, "\ufeff")
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	return strings.ReplaceAll(markdown, "\r", "\n")
}

// RewriteOutsideCode applies replacer to markdown segments that are outside fenced
// code blocks and inline code spans.
func RewriteOutsideCode(markdown string, replacer func(string) string) string {
	if markdown == "" {
		return ""
	}
	markdown = NormalizeLineEndings(markdown)
	lines := strings.SplitAfter(markdown, "\n")
	if len(lines) == 0 {
		return markdown
	}

	var out strings.Builder
	out.Grow(len(markdown))

	inFence := false
	fenceChar := byte(0)
	fenceLen := 0

	for _, line := range lines {
		if inFence {
			out.WriteString(line)
			if isFenceClose(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}

		if ch, n, ok := parseFenceOpen(line); ok {
			inFence = true
			fenceChar = ch
			fenceLen = n
			out.WriteString(line)
			continue
		}

		out.WriteString(rewriteOutsideInlineCode(line, replacer))
	}

	return out.String()
}

// MaskCodeWithSpaces replaces fenced and inline code content with spaces,
// preserving line breaks and string length.
func MaskCodeWithSpaces(markdown string) string {
	if markdown == "" {
		return ""
	}
	markdown = NormalizeLineEndings(markdown)
	lines := strings.SplitAfter(markdown, "\n")
	if len(lines) == 0 {
		return markdown
	}

	var out strings.Builder
	out.Grow(len(markdown))

	inFence := false
	fenceChar := byte(0)
	fenceLen := 0

	for _, line := range lines {
		if inFence {
			out.WriteString(maskKeepNewlines(line))
			if isFenceClose(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}

		if ch, n, ok := parseFenceOpen(line); ok {
			inFence = true
			fenceChar = ch
			fenceLen = n
			out.WriteString(maskKeepNewlines(line))
			continue
		}

		out.WriteString(maskInlineCode(line))
	}

	return out.String()
}

func rewriteOutsideInlineCode(line string, replacer func(string) string) string {
	if line == "" {
		return ""
	}
	var out strings.Builder
	start := 0
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			i++
			continue
		}
		run := countRun(line, i, '`')
		end := findClosingRun(line, i+run, run)
		if end < 0 {
			break
		}
		if start < i {
			out.WriteString(replacer(line[start:i]))
		}
		out.WriteString(line[i : end+run])
		i = end + run
		start = i
	}
	if start < len(line) {
		out.WriteString(replacer(line[start:]))
	}
	return out.String()
}

func maskInlineCode(line string) string {
	if line == "" {
		return ""
	}
	var out strings.Builder
	out.Grow(len(line))
	start := 0
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			i++
			continue
		}
		run := countRun(line, i, '`')
		end := findClosingRun(line, i+run, run)
		if end < 0 {
			break
		}
		if start < i {
			out.WriteString(line[start:i])
		}
		out.WriteString(maskKeepNewlines(line[i : end+run]))
		i = end + run
		start = i
	}
	if start < len(line) {
		out.WriteString(line[start:])
	}
	return out.String()
}

func parseFenceOpen(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return 0, 0, false
	}
	if trimmed[0] != '`' && trimmed[0] != '~' {
		return 0, 0, false
	}
	n := countRun(trimmed, 0, trimmed[0])
	if n < 3 {
		return 0, 0, false
	}
	return trimmed[0], n, true
}

func isFenceClose(line string, ch byte, minLen int) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" || trimmed[0] != ch {
		return false
	}
	n := countRun(trimmed, 0, ch)
	if n < minLen {
		return false
	}
	rest := strings.TrimSpace(trimmed[n:])
	return rest == ""
}

func countRun(s string, start int, ch byte) int {
	n := 0
	for i := start; i < len(s) && s[i] == ch; i++ {
		n++
	}
	return n
}

func findClosingRun(s string, start int, run int) int {
	for i := start; i < len(s); i++ {
		if s[i] != '`' {
			continue
		}
		if countRun(s, i, '`') == run {
			return i
		}
	}
	return -1
}

func maskKeepNewlines(s string) string {
	if s == "" {
		return ""
	}
	buf := []byte(s)
	for i := range buf {
		if buf[i] == '\n' {
			continue
		}
		buf[i] = ' '
	}
	return string(buf)
}
