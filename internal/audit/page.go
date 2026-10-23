package audit

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

type ref struct {
	url    string
	anchor bool // an <a href>, which counts as a link to the page
	image  bool
}

type img struct {
	src    string
	hasAlt bool
}

type page struct {
	route     string
	size      int
	title     string
	meta      map[string]string // name= and property=, lower-cased keys
	canonical string
	hasIcon   bool
	headings  []int
	images    []img
	refs      []ref
	jsonld    int
	jsonldErr string
	textLen   int
	noindex   bool
	redirect  bool
	analytics bool
}

var analyticsMarks = []string{"mc.yandex.ru", "googletagmanager.com", "google-analytics.com", "plausible.io", "umami", "goatcounter", "counter.yadro.ru", "top-fwz1.mail.ru"}

func parsePage(data []byte) *page {
	pg := &page{meta: map[string]string{}}
	z := html.NewTokenizer(bytes.NewReader(data))
	var (
		inTitle, inScript, inStyle, inNoscript bool
		scriptType                             string
		scriptBuf                              strings.Builder
		text                                   int
	)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			attrs := map[string]string{}
			has := map[string]bool{}
			for _, a := range tok.Attr {
				k := strings.ToLower(a.Key)
				attrs[k] = a.Val
				has[k] = true
			}
			switch tok.Data {
			case "title":
				inTitle = tt == html.StartTagToken
			case "meta":
				key := strings.ToLower(firstOf(attrs["name"], attrs["property"]))
				if key != "" {
					if _, seen := pg.meta[key]; !seen {
						pg.meta[key] = attrs["content"]
					}
				}
				if strings.EqualFold(attrs["http-equiv"], "refresh") {
					pg.redirect = true
				}
				if key == "robots" && strings.Contains(strings.ToLower(attrs["content"]), "noindex") {
					pg.noindex = true
				}
			case "link":
				rels := " " + strings.ToLower(attrs["rel"]) + " "
				if strings.Contains(rels, " canonical ") && pg.canonical == "" {
					pg.canonical = strings.TrimSpace(attrs["href"])
				}
				if strings.Contains(rels, " icon ") {
					pg.hasIcon = true
				}
				if attrs["href"] != "" && (strings.Contains(rels, " icon ") || strings.Contains(rels, " stylesheet ") || strings.Contains(rels, " alternate ") && attrs["hreflang"] != "") {
					pg.refs = append(pg.refs, ref{url: attrs["href"]})
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				pg.headings = append(pg.headings, int(tok.Data[1]-'0'))
			case "a":
				if h := attrs["href"]; h != "" {
					pg.refs = append(pg.refs, ref{url: h, anchor: true})
				}
			case "img":
				if inNoscript {
					continue // a tracking pixel, not content
				}
				pg.images = append(pg.images, img{src: attrs["src"], hasAlt: has["alt"]})
				if attrs["src"] != "" {
					pg.refs = append(pg.refs, ref{url: attrs["src"], image: true})
				}
			case "script":
				if tt == html.StartTagToken {
					inScript = true
					scriptType = strings.ToLower(attrs["type"])
					scriptBuf.Reset()
				}
				if src := attrs["src"]; src != "" {
					pg.refs = append(pg.refs, ref{url: src})
					if containsAny(src, analyticsMarks) {
						pg.analytics = true
					}
				}
			case "style":
				inStyle = tt == html.StartTagToken
			case "noscript":
				inNoscript = tt == html.StartTagToken
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "title":
				inTitle = false
			case "script":
				if inScript {
					body := scriptBuf.String()
					if scriptType == "application/ld+json" {
						pg.jsonld++
						if err := validJSON(body); err != nil && pg.jsonldErr == "" {
							pg.jsonldErr = err.Error()
						}
					} else if containsAny(body, analyticsMarks) {
						pg.analytics = true
					}
				}
				inScript = false
			case "style":
				inStyle = false
			case "noscript":
				inNoscript = false
			}
		case html.TextToken:
			t := string(z.Text())
			switch {
			case inTitle:
				pg.title += t
			case inScript:
				scriptBuf.WriteString(t)
			case inStyle:
			default:
				text += utf8.RuneCountInString(strings.Join(strings.Fields(t), " "))
			}
		}
	}
	pg.title = strings.Join(strings.Fields(pg.title), " ")
	pg.textLen = text
	return pg
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func containsAny(s string, marks []string) bool {
	for _, m := range marks {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// imageSize reads the dimensions from the file header: PNG, JPEG and GIF
// through the standard library, WebP by its own header.
func imageSize(file string) (int, int, bool) {
	f, err := os.Open(file)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	if strings.HasSuffix(strings.ToLower(file), ".webp") {
		return webpSize(f)
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

func webpSize(f *os.File) (int, int, bool) {
	head := make([]byte, 30)
	if n, _ := f.Read(head); n < 30 || string(head[0:4]) != "RIFF" || string(head[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(head[12:16]) {
	case "VP8 ":
		w := int(binary.LittleEndian.Uint16(head[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(head[28:30]) & 0x3fff)
		return w, h, true
	case "VP8L":
		b := head[21:25]
		w := 1 + (int(b[0]) | int(b[1]&0x3f)<<8)
		h := 1 + (int(b[1]>>6) | int(b[2])<<2 | int(b[3]&0x0f)<<10)
		return w, h, true
	case "VP8X":
		w := 1 + (int(head[24]) | int(head[25])<<8 | int(head[26])<<16)
		h := 1 + (int(head[27]) | int(head[28])<<8 | int(head[29])<<16)
		return w, h, true
	}
	return 0, 0, false
}
