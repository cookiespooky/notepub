package mediautil

import (
	"net/url"
	"path"
	"strings"
)

func IsExternal(href string) bool {
	lower := strings.ToLower(href)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "//")
}

func EscapePath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func ResolveMediaLink(href, baseKey, prefix, mediaBase string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return href
	}
	if IsExternal(href) {
		return href
	}
	mediaBase = strings.TrimRight(mediaBase, "/")
	if strings.HasPrefix(href, "/media/") {
		if mediaBase == "" {
			return href
		}
		key := strings.TrimPrefix(href, "/media/")
		key = strings.TrimPrefix(key, "/")
		if key == "" {
			return href
		}
		return mediaBase + "/" + EscapePath(key)
	}
	if strings.HasPrefix(href, "/") {
		return href
	}

	baseDir := path.Dir(strings.TrimPrefix(baseKey, "/"))
	if baseDir == "." {
		baseDir = ""
	}
	key := href
	if prefix != "" && strings.HasPrefix(key, prefix) {
		key = strings.TrimPrefix(key, prefix)
		key = strings.TrimPrefix(key, "/")
	} else if baseDir != "" {
		key = path.Join(baseDir, key)
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return href
	}
	if mediaBase == "" {
		return "/media/" + EscapePath(key)
	}
	return mediaBase + "/" + EscapePath(key)
}

func ResolveMediaAbsolute(href, baseKey, prefix, mediaBase, baseURL string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if IsExternal(href) {
		return href
	}
	baseURL = strings.TrimRight(baseURL, "/")
	mediaBase = strings.TrimRight(mediaBase, "/")
	if strings.HasPrefix(href, "/media/") {
		key := strings.TrimPrefix(href, "/media/")
		key = strings.TrimPrefix(key, "/")
		if key == "" {
			return href
		}
		if mediaBase != "" {
			return mediaBase + "/" + EscapePath(key)
		}
		if baseURL != "" {
			return baseURL + "/media/" + EscapePath(key)
		}
		return "/media/" + EscapePath(key)
	}
	if strings.HasPrefix(href, "/") {
		if baseURL == "" {
			return href
		}
		return baseURL + href
	}
	baseDir := path.Dir(strings.TrimPrefix(baseKey, "/"))
	if baseDir == "." {
		baseDir = ""
	}
	key := href
	if prefix != "" && strings.HasPrefix(key, prefix) {
		key = strings.TrimPrefix(key, prefix)
		key = strings.TrimPrefix(key, "/")
	} else if baseDir != "" {
		key = path.Join(baseDir, key)
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return ""
	}
	if mediaBase != "" {
		return mediaBase + "/" + EscapePath(key)
	}
	if baseURL != "" {
		return baseURL + "/media/" + EscapePath(key)
	}
	return "/media/" + EscapePath(key)
}
