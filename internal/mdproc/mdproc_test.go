package mdproc

import (
	"strings"
	"testing"
)

func TestRewriteOutsideCode(t *testing.T) {
	in := strings.Join([]string{
		"Text [[A]] and `[[inline]]`",
		"",
		"```md",
		"[[fenced]]",
		"```",
		"![[img.png]]",
	}, "\n")

	out := RewriteOutsideCode(in, func(seg string) string {
		return strings.ReplaceAll(seg, "[[", "(")
	})

	if !strings.Contains(out, "Text (A]]") {
		t.Fatalf("expected rewrite in plain text, got: %q", out)
	}
	if !strings.Contains(out, "`[[inline]]`") {
		t.Fatalf("inline code must stay unchanged, got: %q", out)
	}
	if !strings.Contains(out, "[[fenced]]") {
		t.Fatalf("fenced code must stay unchanged, got: %q", out)
	}
}

func TestMaskCodeWithSpaces(t *testing.T) {
	in := "A `code` B\n```\nC\n```\nD"
	masked := MaskCodeWithSpaces(in)
	if strings.Contains(masked, "code") {
		t.Fatalf("inline code must be masked: %q", masked)
	}
	if strings.Contains(masked, "C") {
		t.Fatalf("fenced code must be masked: %q", masked)
	}
	if !strings.Contains(masked, "A") || !strings.Contains(masked, "B") || !strings.Contains(masked, "D") {
		t.Fatalf("non-code text must be preserved: %q", masked)
	}
}
