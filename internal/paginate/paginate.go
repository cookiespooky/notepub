// Package paginate holds the arithmetic shared by the indexer, which decides
// how many routes to synthesise, and the renderer, which decides what slice
// each of those routes shows.
//
// It is deliberately tiny and has no dependencies: the two callers live in
// packages that already keep separate copies of the collection filter and sort
// code, and an off-by-one between them would show up as an empty last page or
// a missing article rather than as an error.
package paginate

import (
	"strconv"
	"strings"
)

// PageCount returns how many pages `total` items make at `perPage` each.
// An empty collection still has one page: /blog/ must exist even with no posts.
func PageCount(total, perPage int) int {
	if perPage <= 0 {
		return 1
	}
	if total <= 0 {
		return 1
	}
	n := total / perPage
	if total%perPage != 0 {
		n++
	}
	return n
}

// Bounds returns the half-open range of items shown on page `page` (1-based).
// Out-of-range pages yield an empty range rather than panicking.
func Bounds(total, perPage, page int) (int, int) {
	if perPage <= 0 || page < 1 {
		return 0, total
	}
	start := (page - 1) * perPage
	if start >= total {
		return total, total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return start, end
}

// Path renders a page path template. Both "{{ n }}" and "{{n}}" are accepted,
// matching how permalinks spell their slug placeholder.
func Path(tmpl string, n int) string {
	if tmpl == "" {
		return ""
	}
	num := strconv.Itoa(n)
	out := strings.ReplaceAll(tmpl, "{{ n }}", num)
	out = strings.ReplaceAll(out, "{{n}}", num)
	if !strings.HasPrefix(out, "/") {
		out = "/" + out
	}
	return out
}
