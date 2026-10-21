package serve

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

var monthsRU = []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

// templateFuncs are the helpers every theme can call. Dates in frontmatter
// reach templates as strings ("2025-01-02", "2025-01-02T00:00:00Z") or as
// time values; both are accepted, anything else prints as it is.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// humanDate value lang: "2 января 2025" for ru, "January 2, 2025" otherwise.
		"humanDate": func(v interface{}, lang string) string {
			t, ok := toTime(v)
			if !ok {
				return fmt.Sprint(valueOrEmpty(v))
			}
			if strings.HasPrefix(strings.ToLower(lang), "ru") {
				return fmt.Sprintf("%d %s %d", t.Day(), monthsRU[t.Month()-1], t.Year())
			}
			return t.Format("January 2, 2006")
		},
		// isoDate value: "2025-01-02", for <time datetime>.
		"isoDate": func(v interface{}) string {
			t, ok := toTime(v)
			if !ok {
				return ""
			}
			return t.Format("2006-01-02")
		},
	}
}

func valueOrEmpty(v interface{}) interface{} {
	if v == nil {
		return ""
	}
	return v
}

func toTime(v interface{}) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x.UTC(), !x.IsZero()
	case string:
		x = strings.TrimSpace(x)
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t.UTC(), true
			}
		}
	}
	return time.Time{}, false
}
