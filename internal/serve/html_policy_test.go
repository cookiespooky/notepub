package serve

import (
	"strings"
	"testing"
)

func TestApplyHTMLPolicySafeSanitizes(t *testing.T) {
	in := `<p>x</p><script>alert(1)</script><a href="javascript:alert(1)">bad</a><span onclick="x">y</span>`
	out, report := applyHTMLPolicy(in, "safe")
	if strings.Contains(out, "<script") {
		t.Fatalf("script should be removed: %s", out)
	}
	if strings.Contains(out, "javascript:") {
		t.Fatalf("javascript href should be removed: %s", out)
	}
	if strings.Contains(out, "onclick") {
		t.Fatalf("event attrs should be removed: %s", out)
	}
	if !report.DroppedDangerous {
		t.Fatalf("expected dangerous tag report")
	}
}

func TestApplyHTMLPolicyUnsafeKeepsHTML(t *testing.T) {
	in := `<span data-x="1">ok</span>`
	out, _ := applyHTMLPolicy(in, "unsafe")
	if out != in {
		t.Fatalf("unsafe must keep html, got %q", out)
	}
}

func TestApplyHTMLPolicySafeRestrictsTableCellStyle(t *testing.T) {
	in := `<table><tr><th style="text-align: right; color: red">h</th><td style="background: url(javascript:alert(1))">x</td></tr></table>`
	out, report := applyHTMLPolicy(in, "safe")
	if strings.Contains(out, "color:") || strings.Contains(out, "background:") || strings.Contains(out, "javascript:") {
		t.Fatalf("unsafe style values should be removed: %s", out)
	}
	if !strings.Contains(out, `style="text-align: right"`) {
		t.Fatalf("safe text-align should be preserved: %s", out)
	}
	if strings.Contains(out, `<td style=`) {
		t.Fatalf("td style should be removed when no safe declarations remain: %s", out)
	}
	found := false
	for _, attr := range report.RemovedAttrs {
		if attr == "td.style" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected td.style removal report, got %#v", report.RemovedAttrs)
	}
}

func TestApplyHTMLPolicySafeAllowsVideoEmbed(t *testing.T) {
	in := `<video controls preload="metadata" src="/media/demo.mp4" onloadstart="alert(1)"></video>`
	out, _ := applyHTMLPolicy(in, "safe")
	if !strings.Contains(out, `<video controls="" preload="metadata" src="/media/demo.mp4"></video>`) {
		t.Fatalf("safe video attrs should be preserved: %s", out)
	}
	if strings.Contains(out, "onloadstart") {
		t.Fatalf("event attrs should be removed: %s", out)
	}
}
