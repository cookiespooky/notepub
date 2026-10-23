package audit

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const base = "https://ex.com/docs/"

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePNG(t *testing.T, root, rel string, w, h int) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h)))
}

func head(title, canonical, ogImage string) string {
	return `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + title + `</title>
<meta name="description" content="A description long enough to pass the minimum length check, clearly.">
<link rel="canonical" href="` + canonical + `">
<link rel="icon" href="/docs/favicon.ico">
<meta property="og:title" content="t"><meta property="og:description" content="d">
<meta property="og:url" content="` + canonical + `"><meta property="og:image" content="` + ogImage + `">
<script type="application/ld+json">{"@type":"WebPage"}</script>
<script src="https://mc.yandex.ru/metrika/tag.js"></script>
</head><body>`
}

const filler = `<p>Text long enough to count as content on a page, written in the HTML itself so that crawlers read it without running scripts at all.</p>`

func fixture(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "favicon.ico", "x")
	writePNG(t, root, "og.png", 1200, 630)
	writePNG(t, root, "small.png", 600, 600)
	write(t, root, "index.html", head("Home", base, base+"og.png")+`<h1>Home</h1>`+filler+
		`<a href="/docs/guide/">guide</a> <a href="/docs/about">about</a> <a href="/docs/gone/">gone</a>
<a href="/other-site/page/">other</a> <a href="https://elsewhere.org/x">ext</a> <img src="/docs/og.png"></body></html>`)
	write(t, root, "guide/index.html", head("Guide", base+"guide/", base+"small.png")+`<h1>Guide</h1><h3>skip</h3>`+filler+
		`<a href="/docs/">home</a></body></html>`)
	write(t, root, "about/index.html", head("Guide", base+"about/", base+"og.png")+`<h1>About</h1>`+filler+`</body></html>`)
	write(t, root, "lonely/index.html", head("Lonely", base+"lonely/", base+"og.png")+`<h1>Lonely</h1>`+filler+`</body></html>`)
	write(t, root, "hidden/index.html", `<html><head><meta name="robots" content="noindex"><title>x</title></head><body></body></html>`)
	write(t, root, "googleabc123.html", "google-site-verification: googleabc123.html")
	write(t, root, "404.html", `<section><h1>Not found</h1></section>`)
	write(t, root, "robots.txt", "User-agent: *\nAllow: /\n")
	write(t, root, "sitemap.xml", `<urlset><url><loc>`+base+`</loc></url><url><loc>`+base+`guide/</loc></url><url><loc>`+base+`hidden/</loc></url><url><loc>`+base+`about</loc></url></urlset>`)
	return root
}

func find(r Report, code, path string) *Finding {
	for i, f := range r.Findings {
		if f.Code == code && (path == "*" || f.Path == path) {
			return &r.Findings[i]
		}
	}
	return nil
}

func TestAuditFindsWhatTheBuildGotWrong(t *testing.T) {
	r, err := Run(Options{Dist: fixture(t), BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ code, path, sev string }{
		{"link-redirect", "/", "warning"},        // /docs/about without slash
		{"link-broken", "/", "error"},            // /docs/gone/
		{"link-outside-build", "/", "info"},      // /other-site/ is not under /docs/ — skipped? no: different base path
		{"og-image-small", "/guide/", "warning"}, // 600×600
		{"heading-skip", "/guide/", "info"},
		{"title-duplicate", "/about/", "warning"},
		{"page-orphan", "/lonely/", "warning"},
		{"robots-no-sitemap", "", "warning"},
		{"robots-under-path", "", "info"},
		{"sitemap-noindex", "/hidden/", "warning"},
		{"sitemap-broken", "/about", "error"},
		{"sitemap-absent", "/lonely/", "info"},
		{"404-bare", "/404.html", "warning"},
		{"llms-missing", "", "info"},
	}
	for _, w := range want {
		if w.code == "link-outside-build" {
			continue // /other-site/ is outside /docs/, so it is another site and not reported at all
		}
		f := find(r, w.code, w.path)
		if f == nil {
			t.Errorf("expected %s on %q; got:\n%s", w.code, w.path, dump(r))
			continue
		}
		if f.Severity != w.sev {
			t.Errorf("%s on %q: severity %s, want %s", w.code, w.path, f.Severity, w.sev)
		}
	}
	for _, unwanted := range []struct{ code, path string }{
		{"og-image-small", "/"},
		{"page-orphan", "/guide/"},
		{"page-orphan", "/about/"},
		{"analytics-missing", ""},
		{"title-missing", "*"},
		{"link-broken", "/guide/"},
	} {
		if f := find(r, unwanted.code, unwanted.path); f != nil {
			t.Errorf("unexpected %s on %q: %s", f.Code, f.Path, f.Message)
		}
	}
	for _, f := range r.Findings {
		if strings.Contains(f.Path, "google") || strings.Contains(f.Message, "elsewhere.org") || strings.Contains(f.Message, "other-site") {
			t.Errorf("must ignore verification files, external sites and other sites on the domain: %+v", f)
		}
		if f.Path == "/hidden/" && f.Code != "sitemap-noindex" {
			t.Errorf("a noindex page gets no page checks: %+v", f)
		}
	}
}

func TestAuditLinkIntoMissingSectionAtRootIsInfo(t *testing.T) {
	root := t.TempDir()
	write(t, root, "index.html", head("Home", "https://ex.com/", "https://ex.com/og.png")+`<h1>H</h1>`+filler+
		`<a href="/np/en/">engine docs</a> <a href="/blog/typo/">typo</a></body></html>`)
	write(t, root, "blog/index.html", head("Blog", "https://ex.com/blog/", "https://ex.com/og.png")+`<h1>B</h1>`+filler+`<a href="/">h</a></body></html>`)
	r, err := Run(Options{Dist: root, BaseURL: "https://ex.com"})
	if err != nil {
		t.Fatal(err)
	}
	if f := findMsg(r, "link-outside-build", "/np/en/"); f == nil || f.Severity != "info" {
		t.Errorf("a folder the build never wrote is another site, info only:\n%s", dump(r))
	}
	if f := findMsg(r, "link-broken", "/blog/typo/"); f == nil || f.Severity != "error" {
		t.Errorf("a missing page inside a built section is broken:\n%s", dump(r))
	}
}

func TestWebPSize(t *testing.T) {
	root := t.TempDir()
	// VP8X header: canvas 1200×630 stored as width-1 and height-1, 24-bit little endian.
	hdr := []byte("RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x00\x00\x00\x00")
	w, h := 1199, 629
	hdr = append(hdr, byte(w), byte(w>>8), byte(w>>16), byte(h), byte(h>>8), byte(h>>16))
	p := filepath.Join(root, "a.webp")
	os.WriteFile(p, hdr, 0o644)
	gw, gh, ok := imageSize(p)
	if !ok || gw != 1200 || gh != 630 {
		t.Fatalf("got %d×%d ok=%v", gw, gh, ok)
	}
}

func findMsg(r Report, code, part string) *Finding {
	for i, f := range r.Findings {
		if f.Code == code && strings.Contains(f.Message, part) {
			return &r.Findings[i]
		}
	}
	return nil
}

func dump(r Report) string {
	var b strings.Builder
	for _, f := range r.Findings {
		b.WriteString(f.Severity + " " + f.Code + " " + f.Path + " " + f.Message + "\n")
	}
	return b.String()
}
