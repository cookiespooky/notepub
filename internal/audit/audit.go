// Package audit checks a built site — the dist folder, as it will be served —
// against an SEO checklist. It needs no network: everything it reports can be
// seen in the files. What only the host decides (HTTPS, headers, redirects
// between www and the bare domain) is outside it.
//
// The index-level check (`validate --seo`) sees notes; this one sees pages,
// menus and templates included, which is where a link that only redirects, a
// 404 page without <head> or a square og:image come from.
package audit

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	titleMax       = 70
	descriptionMin = 50
	descriptionMax = 200
	// og:image is shown large from 1200×630; smaller images become a thumbnail.
	ogMinWidth  = 1200
	ogMinHeight = 600
	imageMaxKB  = 500
	htmlMaxKB   = 300
)

// Finding is one result. Path is the page's address relative to the site root
// ("/blog/post/"), or empty for the whole site.
type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error, warning, info
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

// Options says where the site is and where it will live.
type Options struct {
	Dist    string
	BaseURL string // the address the site is served at, e.g. https://example.com/docs/
}

// Report is what Run returns.
type Report struct {
	Pages    int       `json:"pages"`
	Findings []Finding `json:"findings"`
}

func (r Report) Count(severity string) int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == severity {
			n++
		}
	}
	return n
}

type site struct {
	dist     string
	base     *url.URL
	basePath string // "/docs/" or "/"
	pages    map[string]*page
	findings []Finding
	inbound  map[string]map[string]bool
}

// Run reads every HTML file under opts.Dist and checks pages and site files.
func Run(opts Options) (Report, error) {
	base, err := url.Parse(strings.TrimSpace(opts.BaseURL))
	if err != nil || base.Host == "" {
		return Report{}, fmt.Errorf("base URL must be absolute, got %q", opts.BaseURL)
	}
	if info, err := os.Stat(opts.Dist); err != nil || !info.IsDir() {
		return Report{}, fmt.Errorf("dist folder not found: %s", opts.Dist)
	}
	bp := "/" + strings.Trim(base.Path, "/") + "/"
	if bp == "//" {
		bp = "/"
	}
	s := &site{dist: opts.Dist, base: base, basePath: bp, pages: map[string]*page{}, inbound: map[string]map[string]bool{}}

	err = filepath.WalkDir(opts.Dist, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		rel, _ := filepath.Rel(opts.Dist, p)
		rel = filepath.ToSlash(rel)
		if rel == "404.html" || verificationFile.MatchString(rel) {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		route := "/" + strings.TrimSuffix(strings.TrimSuffix(rel, "index.html"), ".html")
		pg := parsePage(data)
		pg.route = route
		pg.size = len(data)
		s.pages[route] = pg
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	routes := make([]string, 0, len(s.pages))
	for r := range s.pages {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	indexable := 0
	for _, r := range routes {
		pg := s.pages[r]
		if pg.redirect {
			continue
		}
		s.checkLinks(pg)
		if pg.noindex {
			continue
		}
		indexable++
		s.checkPage(pg)
	}
	s.checkDuplicates(routes)
	s.checkOrphans(routes)
	s.checkSiteFiles()
	s.checkNotFound()

	sort.SliceStable(s.findings, func(i, j int) bool {
		if s.findings[i].Path != s.findings[j].Path {
			return s.findings[i].Path < s.findings[j].Path
		}
		return s.findings[i].Code < s.findings[j].Code
	})
	return Report{Pages: indexable, Findings: s.findings}, nil
}

func (s *site) add(sev, code, route, msg string) {
	s.findings = append(s.findings, Finding{Code: code, Severity: sev, Path: route, Message: msg})
}

// pageURL is the absolute address a page is served at.
func (s *site) pageURL(route string) string {
	u := *s.base
	u.Path = path.Join(s.basePath, route)
	if strings.HasSuffix(route, "/") && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

func (s *site) checkPage(pg *page) {
	r := pg.route
	title := strings.TrimSpace(pg.title)
	switch n := utf8.RuneCountInString(title); {
	case n == 0:
		s.add("error", "title-missing", r, "no <title>")
	case n > titleMax:
		s.add("warning", "title-long", r, fmt.Sprintf("title is %d characters; search results show about %d", n, titleMax))
	}
	desc := strings.TrimSpace(pg.meta["description"])
	switch n := utf8.RuneCountInString(desc); {
	case n == 0:
		s.add("warning", "description-missing", r, "no meta description")
	case n < descriptionMin:
		s.add("info", "description-short", r, fmt.Sprintf("description is %d characters; %d–%d works best", n, descriptionMin, descriptionMax))
	case n > descriptionMax:
		s.add("warning", "description-long", r, fmt.Sprintf("description is %d characters and will be cut", n))
	}

	h1 := 0
	prev := 0
	skipped := ""
	for _, h := range pg.headings {
		if h == 1 {
			h1++
		}
		if prev > 0 && h > prev+1 && skipped == "" {
			skipped = fmt.Sprintf("h%d is followed by h%d", prev, h)
		}
		prev = h
	}
	switch {
	case h1 == 0:
		s.add("warning", "h1-missing", r, "no <h1>")
	case h1 > 1:
		s.add("warning", "h1-multiple", r, fmt.Sprintf("%d <h1> on the page", h1))
	}
	if skipped != "" {
		s.add("info", "heading-skip", r, skipped+": a heading level is skipped")
	}

	own := s.pageURL(r)
	switch c := pg.canonical; {
	case c == "":
		s.add("warning", "canonical-missing", r, "no <link rel=\"canonical\">")
	case !isAbsolute(c):
		s.add("warning", "canonical-relative", r, "canonical is not an absolute address: "+c)
	case hostOf(c) != strings.ToLower(s.base.Host):
		s.add("warning", "canonical-host", r, "canonical points to another host ("+hostOf(c)+"); a local base_url left in the build?")
	case c != own:
		s.add("info", "canonical-other", r, "canonical is "+c+", not this page's own address")
	}

	for _, k := range []string{"og:title", "og:description", "og:image", "og:url"} {
		if strings.TrimSpace(pg.meta[k]) == "" {
			s.add("warning", "og-missing", r, "no "+k+": a link to the page is shared without it")
		}
	}
	if img := strings.TrimSpace(pg.meta["og:image"]); img != "" {
		if !isAbsolute(img) {
			s.add("warning", "og-image-relative", r, "og:image must be an absolute address: "+img)
		} else if file, ok := s.localFile(img); ok {
			if w, h, ok := imageSize(file); ok && (w < ogMinWidth || h < ogMinHeight) {
				s.add("warning", "og-image-small", r, fmt.Sprintf("og:image is %d×%d; previews show it large from 1200×630", w, h))
			}
		}
	}

	if !pg.hasIcon {
		s.add("warning", "favicon-missing", r, "no <link rel=\"icon\">")
	}
	if !strings.Contains(pg.meta["viewport"], "width=device-width") {
		s.add("warning", "viewport-missing", r, "no viewport meta with width=device-width: the page is not laid out for phones")
	}
	if pg.jsonldErr != "" {
		s.add("error", "jsonld-invalid", r, "JSON-LD does not parse: "+pg.jsonldErr)
	} else if pg.jsonld == 0 {
		s.add("info", "jsonld-missing", r, "no schema.org markup (JSON-LD)")
	}
	if pg.textLen < 200 {
		s.add("info", "text-thin", r, fmt.Sprintf("only %d characters of text in the HTML itself", pg.textLen))
	}
	missingAlt := 0
	for _, img := range pg.images {
		if !img.hasAlt {
			missingAlt++
		}
	}
	if missingAlt > 0 {
		s.add("warning", "img-alt-missing", r, fmt.Sprintf("%d image(s) without an alt attribute", missingAlt))
	}
	if pg.size > htmlMaxKB*1024 {
		s.add("info", "html-heavy", r, fmt.Sprintf("the HTML weighs %d KB", pg.size/1024))
	}
}

// checkLinks resolves every link and image of a page that stays on this site.
func (s *site) checkLinks(pg *page) {
	broken := map[string]bool{}
	outside := map[string]bool{}
	redirects := map[string]bool{}
	heavy := map[string]bool{}
	for _, ref := range pg.refs {
		target, ok := s.resolve(pg.route, ref.url)
		if !ok {
			continue
		}
		file, kind := s.lookup(target)
		if kind == "missing" && !s.sectionExists(target) {
			outside[ref.url] = true
			continue
		}
		switch kind {
		case "missing":
			broken[ref.url] = true
		case "slash":
			redirects[ref.url] = true
			// The redirect still lands on the page: it is linked, just slowly.
			s.link(pg.route, target+"/", ref.anchor)
		case "page":
			s.link(pg.route, target, ref.anchor)
		case "file":
			if ref.image {
				if info, err := os.Stat(file); err == nil && info.Size() > imageMaxKB*1024 {
					heavy[ref.url] = true
				}
			}
		}
	}
	for _, u := range sortedKeys(broken) {
		s.add("error", "link-broken", pg.route, "leads nowhere: "+u)
	}
	for _, u := range sortedKeys(outside) {
		s.add("info", "link-outside-build", pg.route, "not in this build — another site under the same domain? Check it there: "+u)
	}
	for _, u := range sortedKeys(redirects) {
		s.add("warning", "link-redirect", pg.route, "no trailing slash, the host answers with a redirect: "+u)
	}
	for _, u := range sortedKeys(heavy) {
		s.add("warning", "image-heavy", pg.route, fmt.Sprintf("image over %d KB: %s", imageMaxKB, u))
	}
}

func (s *site) link(from, to string, anchor bool) {
	if to == from || !anchor {
		return
	}
	if s.inbound[to] == nil {
		s.inbound[to] = map[string]bool{}
	}
	s.inbound[to][from] = true
}

// resolve turns a reference into a path under the site root ("/a/b/"),
// or reports false for addresses on other sites and non-HTTP schemes.
func (s *site) resolve(from, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") {
		return "", false
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	pageURL, _ := url.Parse(s.pageURL(from))
	abs := pageURL.ResolveReference(u)
	if !strings.EqualFold(abs.Host, s.base.Host) {
		return "", false
	}
	p := abs.Path
	if !strings.HasPrefix(p+"/", s.basePath) {
		return "", false // same host, outside this site (another site under the domain)
	}
	rel := "/" + strings.TrimPrefix(p, s.basePath)
	if p+"/" == s.basePath {
		rel = "/"
	}
	return rel, true
}

// lookup says what a site path is in dist: a page, a file, a folder that
// static hosts reach only after a redirect to the slashed address, or nothing.
func (s *site) lookup(rel string) (string, string) {
	fsPath := filepath.Join(s.dist, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
	if strings.HasSuffix(rel, "/") {
		if _, err := os.Stat(filepath.Join(fsPath, "index.html")); err == nil {
			return "", "page"
		}
		return "", "missing"
	}
	info, err := os.Stat(fsPath)
	if err == nil && info.IsDir() {
		if _, err := os.Stat(filepath.Join(fsPath, "index.html")); err == nil {
			return "", "slash"
		}
		return "", "missing"
	}
	if err == nil {
		if strings.HasSuffix(rel, ".html") {
			return fsPath, "page"
		}
		return fsPath, "file"
	}
	return "", "missing"
}

// sectionExists says whether the first folder of a path is in dist at all. A
// link into a folder the build never wrote most likely points to another site
// deployed under the same domain (a GitHub Pages project site under a user
// site's domain), which this check cannot see.
func (s *site) sectionExists(rel string) bool {
	if s.basePath != "/" {
		return true // a site under a path owns everything below it
	}
	first := strings.SplitN(strings.Trim(rel, "/"), "/", 2)[0]
	if first == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(s.dist, first))
	return err == nil
}

// localFile maps an absolute address on this site to its file in dist.
func (s *site) localFile(abs string) (string, bool) {
	rel, ok := s.resolve("/", abs)
	if !ok {
		return "", false
	}
	file, kind := s.lookup(rel)
	return file, kind == "file"
}

func (s *site) checkDuplicates(routes []string) {
	titles := map[string][]string{}
	descs := map[string][]string{}
	for _, r := range routes {
		pg := s.pages[r]
		if pg.redirect || pg.noindex {
			continue
		}
		if t := strings.ToLower(strings.TrimSpace(pg.title)); t != "" {
			titles[t] = append(titles[t], r)
		}
		if d := strings.ToLower(strings.TrimSpace(pg.meta["description"])); d != "" {
			descs[d] = append(descs[d], r)
		}
	}
	for _, group := range groups(titles) {
		for _, r := range group {
			s.add("warning", "title-duplicate", r, "same title as "+strings.Join(without(group, r), ", "))
		}
	}
	for _, group := range groups(descs) {
		for _, r := range group {
			s.add("warning", "description-duplicate", r, "same description as "+strings.Join(without(group, r), ", "))
		}
	}
}

// checkOrphans reports pages no other page links to — menus included, unlike
// the check on notes.
func (s *site) checkOrphans(routes []string) {
	for _, r := range routes {
		pg := s.pages[r]
		if r == "/" || pg.redirect || pg.noindex {
			continue
		}
		if len(s.inbound[r]) == 0 {
			s.add("warning", "page-orphan", r, "no other page links here: visitors and crawlers find it only through the sitemap, if at all")
		}
	}
}

// verificationFile matches the files search consoles ask to upload to prove
// ownership: they are not pages.
var verificationFile = regexp.MustCompile(`^(google[0-9a-f]+|yandex_[0-9a-f]+|BingSiteAuth)\.(html|xml)$`)

var sitemapLoc = regexp.MustCompile(`<loc>\s*([^<\s]+)\s*</loc>`)

func (s *site) checkSiteFiles() {
	robots, err := os.ReadFile(filepath.Join(s.dist, "robots.txt"))
	switch {
	case err != nil:
		s.add("warning", "robots-missing", "", "no robots.txt")
	case !regexp.MustCompile(`(?mi)^\s*sitemap:`).Match(robots):
		s.add("warning", "robots-no-sitemap", "", "robots.txt does not name the sitemap")
	}
	if s.basePath != "/" {
		s.add("info", "robots-under-path", "", fmt.Sprintf("the site lives under %s: crawlers read robots.txt only at %s://%s/robots.txt — the sitemap must be named there", s.basePath, s.base.Scheme, s.base.Host))
	}

	locs := s.sitemapURLs()
	if locs == nil {
		s.add("warning", "sitemap-missing", "", "no sitemap.xml or sitemap-index.xml")
	} else {
		listed := map[string]bool{}
		for _, loc := range locs {
			rel, ok := s.resolve("/", loc)
			if !ok {
				s.add("warning", "sitemap-offsite", "", "sitemap lists an address outside the site: "+loc)
				continue
			}
			listed[rel] = true
			if _, kind := s.lookup(rel); kind != "page" {
				s.add("error", "sitemap-broken", rel, "listed in the sitemap, but "+map[string]string{"missing": "there is no such page", "slash": "only without the trailing slash it redirects", "file": "it is a file, not a page"}[kind])
			} else if pg := s.pages[rel]; pg != nil && pg.noindex {
				s.add("warning", "sitemap-noindex", rel, "listed in the sitemap but marked noindex")
			}
		}
		for r, pg := range s.pages {
			if pg.redirect || pg.noindex || listed[r] {
				continue
			}
			s.add("info", "sitemap-absent", r, "an indexable page the sitemap does not list")
		}
	}

	if _, err := os.Stat(filepath.Join(s.dist, "llms.txt")); err != nil {
		s.add("info", "llms-missing", "", "no llms.txt (set llms.enabled to have the engine write it)")
	} else if data, err := os.ReadFile(filepath.Join(s.dist, "llms.txt")); err == nil {
		for _, m := range regexp.MustCompile(`https?://[^\s)>\]]+`).FindAllString(string(data), -1) {
			rel, ok := s.resolve("/", m)
			if !ok {
				continue
			}
			if _, kind := s.lookup(rel); kind == "missing" {
				s.add("warning", "llms-broken", "", "llms.txt links to a page that is not there: "+m)
			} else if kind == "slash" {
				s.add("info", "llms-redirect", "", "llms.txt links without the trailing slash: "+m)
			}
		}
	}

	analytics := false
	for _, pg := range s.pages {
		if pg.analytics {
			analytics = true
			break
		}
	}
	if !analytics {
		s.add("info", "analytics-missing", "", "no Yandex Metrica, Google Analytics or similar counter found on any page")
	}
}

func (s *site) sitemapURLs() []string {
	var out []string
	found := false
	var read func(name string, depth int)
	read = func(name string, depth int) {
		data, err := os.ReadFile(filepath.Join(s.dist, name))
		if err != nil || depth > 2 {
			return
		}
		found = true
		for _, m := range sitemapLoc.FindAllStringSubmatch(string(data), -1) {
			loc := m[1]
			if strings.Contains(string(data), "<sitemapindex") {
				if rel, ok := s.resolve("/", loc); ok {
					read(strings.TrimPrefix(rel, "/"), depth+1)
				}
				continue
			}
			out = append(out, loc)
		}
	}
	read("sitemap.xml", 0)
	if !found {
		read("sitemap-index.xml", 0)
	}
	if !found {
		return nil
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func (s *site) checkNotFound() {
	data, err := os.ReadFile(filepath.Join(s.dist, "404.html"))
	if err != nil {
		s.add("warning", "404-missing", "", "no 404.html: the host shows its own error page")
		return
	}
	pg := parsePage(data)
	low := strings.ToLower(string(data))
	if !strings.Contains(low, "<html") || !strings.Contains(low, "<head") || strings.TrimSpace(pg.title) == "" {
		s.add("warning", "404-bare", "/404.html", "404.html is not a full document (no <html>, <head> or <title>): it shows unstyled")
	}
	for _, ref := range pg.refs {
		if rel, ok := s.resolve("/", ref.url); ok {
			if _, kind := s.lookup(rel); kind == "missing" {
				s.add("warning", "404-link-broken", "/404.html", "a link on the 404 page leads nowhere: "+ref.url)
			}
		}
	}
}

func isAbsolute(u string) bool {
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return strings.ToLower(p.Host)
}

func groups(m map[string][]string) [][]string {
	out := [][]string{}
	for _, g := range m {
		if len(g) > 1 {
			sort.Strings(g)
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func without(group []string, self string) []string {
	out := []string{}
	for _, g := range group {
		if g != self {
			out = append(out, g)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func validJSON(s string) error {
	var v interface{}
	return json.Unmarshal([]byte(s), &v)
}
