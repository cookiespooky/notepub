package mediautil

import (
	"net/url"
	"path"
	"sort"
	"strings"
)

// Finder resolves an Obsidian embed (![[photo.jpg]]) the way Obsidian does:
// next to the note, then from the vault root, then by file name anywhere.
// Keys are paths under /media/, as the build publishes them.
type Finder struct {
	byPath map[string]string
	byName map[string][]string
}

func NewFinder(keys []string) *Finder {
	f := &Finder{byPath: map[string]string{}, byName: map[string][]string{}}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for _, key := range sorted {
		key = strings.TrimPrefix(key, "/")
		if key == "" {
			continue
		}
		f.byPath[strings.ToLower(key)] = key
		name := strings.ToLower(path.Base(key))
		f.byName[name] = append(f.byName[name], key)
	}
	return f
}

// Len is the number of known media files; a finder with none changes nothing.
func (f *Finder) Len() int {
	if f == nil {
		return 0
	}
	return len(f.byPath)
}

// Find returns the media key of target as embedded in the note at baseKey.
// With several files of one name the shortest path wins, as in Obsidian.
func (f *Finder) Find(target, baseKey, prefix string) (string, bool) {
	if f.Len() == 0 {
		return "", false
	}
	target = strings.TrimSpace(target)
	if target == "" || IsExternal(target) {
		return "", false
	}
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	target = strings.TrimPrefix(target, "/")
	target = strings.TrimPrefix(target, "media/")
	baseKey = strings.TrimPrefix(strings.TrimPrefix(baseKey, prefix), "/")
	baseDir := path.Dir(baseKey)
	if baseDir == "." {
		baseDir = ""
	}
	for _, candidate := range []string{path.Join(baseDir, target), path.Clean(target)} {
		if key, ok := f.byPath[strings.ToLower(strings.TrimPrefix(candidate, "/"))]; ok {
			return key, true
		}
	}
	matches := f.byName[strings.ToLower(path.Base(target))]
	if len(matches) == 0 {
		return "", false
	}
	best := matches[0]
	for _, m := range matches[1:] {
		if strings.Count(m, "/") < strings.Count(best, "/") {
			best = m
		}
	}
	return best, true
}
