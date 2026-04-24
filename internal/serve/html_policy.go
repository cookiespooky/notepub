package serve

import (
	"bytes"
	"html"
	"strings"

	htmlnode "golang.org/x/net/html"
)

type htmlSanitizeReport struct {
	RemovedTags      []string
	RemovedAttrs     []string
	DroppedDangerous bool
}

func applyHTMLPolicy(body string, policy string) (string, htmlSanitizeReport) {
	report := htmlSanitizeReport{}
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "unsafe":
		return body, report
	case "deny", "safe", "":
		return sanitizeHTML(body)
	default:
		return sanitizeHTML(body)
	}
}

func sanitizeHTML(body string) (string, htmlSanitizeReport) {
	report := htmlSanitizeReport{}
	if strings.TrimSpace(body) == "" {
		return body, report
	}
	doc, err := htmlnode.Parse(strings.NewReader("<div>" + body + "</div>"))
	if err != nil {
		return body, report
	}
	root := findFirstElementByTag(doc, "div")
	if root == nil {
		return body, report
	}
	var out bytes.Buffer
	for n := root.FirstChild; n != nil; n = n.NextSibling {
		sanitizeNode(&out, n, &report)
	}
	return out.String(), report
}

func findFirstElementByTag(n *htmlnode.Node, tag string) *htmlnode.Node {
	if n == nil {
		return nil
	}
	if n.Type == htmlnode.ElementNode && strings.EqualFold(n.Data, tag) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hit := findFirstElementByTag(c, tag); hit != nil {
			return hit
		}
	}
	return nil
}

func sanitizeNode(out *bytes.Buffer, n *htmlnode.Node, report *htmlSanitizeReport) {
	switch n.Type {
	case htmlnode.TextNode:
		out.WriteString(html.EscapeString(n.Data))
		return
	case htmlnode.CommentNode:
		return
	case htmlnode.ElementNode:
		tag := strings.ToLower(strings.TrimSpace(n.Data))
		if tag == "script" || tag == "style" || tag == "iframe" || tag == "object" || tag == "embed" {
			report.DroppedDangerous = true
			report.RemovedTags = appendIfMissing(report.RemovedTags, tag)
			return
		}
		if !isAllowedTag(tag) {
			report.RemovedTags = appendIfMissing(report.RemovedTags, tag)
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				sanitizeNode(out, c, report)
			}
			return
		}
		out.WriteString("<")
		out.WriteString(tag)
		for _, a := range n.Attr {
			key := strings.ToLower(strings.TrimSpace(a.Key))
			val := strings.TrimSpace(a.Val)
			if !isAllowedAttr(tag, key) {
				report.RemovedAttrs = appendIfMissing(report.RemovedAttrs, tag+"."+key)
				continue
			}
			if (key == "href" || key == "src") && !isAllowedURL(val) {
				report.RemovedAttrs = appendIfMissing(report.RemovedAttrs, tag+"."+key)
				continue
			}
			if key == "style" {
				style, ok := sanitizeTableCellStyle(tag, val)
				if !ok {
					report.RemovedAttrs = appendIfMissing(report.RemovedAttrs, tag+"."+key)
					continue
				}
				val = style
			}
			out.WriteString(" ")
			out.WriteString(key)
			out.WriteString("=\"")
			out.WriteString(html.EscapeString(val))
			out.WriteString("\"")
		}
		if isVoidTag(tag) {
			out.WriteString(">")
			return
		}
		out.WriteString(">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sanitizeNode(out, c, report)
		}
		out.WriteString("</")
		out.WriteString(tag)
		out.WriteString(">")
		return
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sanitizeNode(out, c, report)
		}
	}
}

func isAllowedTag(tag string) bool {
	_, ok := allowedHTMLTags[tag]
	return ok
}

func isAllowedAttr(tag string, attr string) bool {
	if attr == "" {
		return false
	}
	if _, ok := allowedGlobalAttrs[attr]; ok {
		return true
	}
	if attrs, ok := allowedTagAttrs[tag]; ok {
		if _, ok := attrs[attr]; ok {
			return true
		}
	}
	return false
}

func isAllowedURL(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "#") || strings.HasPrefix(v, "/") || strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") {
		return true
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "mailto:") {
		return true
	}
	return false
}

func sanitizeTableCellStyle(tag string, v string) (string, bool) {
	if tag != "td" && tag != "th" {
		return "", false
	}
	var kept []string
	for _, decl := range strings.Split(v, ";") {
		parts := strings.SplitN(decl, ":", 2)
		if len(parts) != 2 {
			continue
		}
		prop := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.ToLower(strings.TrimSpace(parts[1]))
		if prop != "text-align" {
			continue
		}
		switch value {
		case "left", "right", "center", "justify", "start", "end":
			kept = append(kept, prop+": "+value)
		}
	}
	if len(kept) == 0 {
		return "", false
	}
	return strings.Join(kept, "; "), true
}

func isVoidTag(tag string) bool {
	switch tag {
	case "img", "br", "hr", "input":
		return true
	default:
		return false
	}
}

func appendIfMissing(items []string, value string) []string {
	for _, v := range items {
		if v == value {
			return items
		}
	}
	return append(items, value)
}

var (
	allowedHTMLTags = map[string]struct{}{
		"a": {}, "abbr": {}, "b": {}, "blockquote": {}, "br": {}, "code": {}, "del": {}, "details": {}, "div": {},
		"em": {}, "h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {}, "hr": {}, "img": {}, "input": {},
		"li": {}, "mark": {}, "ol": {}, "p": {}, "pre": {}, "section": {}, "small": {}, "span": {}, "strong": {},
		"sub": {}, "summary": {}, "sup": {}, "table": {}, "tbody": {}, "td": {}, "th": {}, "thead": {}, "tr": {},
		"ul":    {},
		"video": {},
	}
	allowedGlobalAttrs = map[string]struct{}{
		"id": {}, "class": {}, "role": {}, "aria-label": {}, "aria-hidden": {}, "title": {},
	}
	allowedTagAttrs = map[string]map[string]struct{}{
		"a":       {"href": {}, "target": {}, "rel": {}},
		"img":     {"src": {}, "alt": {}, "width": {}, "height": {}, "loading": {}},
		"input":   {"type": {}, "checked": {}, "disabled": {}},
		"details": {"open": {}},
		"td":      {"colspan": {}, "rowspan": {}, "style": {}},
		"th":      {"colspan": {}, "rowspan": {}, "style": {}},
		"video":   {"src": {}, "controls": {}, "preload": {}, "poster": {}, "width": {}, "height": {}},
	}
)
