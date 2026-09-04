package urlutil

import (
	"net/url"
	"path"
	"strings"
)

// JoinBaseURL combines a base URL (which may include a path) with a route path.
// It preserves the base path and ensures "/" routes render with a trailing slash.
func JoinBaseURL(baseURL, routePath string) string {
	baseURL = strings.TrimSpace(baseURL)
	routePath = strings.TrimSpace(routePath)
	if baseURL == "" {
		return routePath
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		baseURL = strings.TrimRight(baseURL, "/")
		if routePath == "" || routePath == "/" {
			return baseURL + "/"
		}
		if strings.HasPrefix(routePath, "/") {
			return baseURL + routePath
		}
		return baseURL + "/" + routePath
	}

	basePath := strings.TrimRight(u.Path, "/")
	if routePath == "" || routePath == "/" {
		if basePath == "" {
			u.Path = "/"
		} else {
			u.Path = basePath + "/"
		}
		return u.String()
	}

	if !strings.HasPrefix(routePath, "/") {
		routePath = "/" + routePath
	}
	joined := ""
	if basePath == "" {
		joined = path.Clean(routePath)
	} else {
		joined = path.Clean(basePath + "/" + strings.TrimPrefix(routePath, "/"))
	}
	if strings.HasSuffix(routePath, "/") && joined != "/" && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	u.Path = joined
	return u.String()
}

// PublicPath returns the browsable form of a route path.
//
// The static builder writes every route as <path>/index.html, so the address a
// visitor or a crawler actually lands on always ends with a slash. Route keys,
// by contrast, are stored trimmed because request matching normalises the same
// way. Keeping the two apart is what stops canonical links and sitemap entries
// from pointing at an address that only redirects to the real one.
func PublicPath(routePath string) string {
	p := strings.TrimSpace(routePath)
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if strings.HasSuffix(p, "/") {
		return p
	}
	return p + "/"
}
