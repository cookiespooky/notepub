package serve

import (
	"testing"

	"github.com/cookiespooky/notepub/internal/models"
)

func TestResolveRoutePath(t *testing.T) {
	routes := map[string]models.RouteEntry{
		"/":       {Status: 200},
		"/note":   {Status: 200},
		"/legacy/": {Status: 200},
	}

	t.Run("root", func(t *testing.T) {
		got, _, ok := resolveRoutePath(routes, "/")
		if !ok || got != "/" {
			t.Fatalf("resolveRoutePath(/) = (%q, %v), want (/ , true)", got, ok)
		}
	})

	t.Run("trim trailing slash", func(t *testing.T) {
		got, _, ok := resolveRoutePath(routes, "/note/")
		if !ok || got != "/note" {
			t.Fatalf("resolveRoutePath(/note/) = (%q, %v), want (/note , true)", got, ok)
		}
	})

	t.Run("add trailing slash for legacy route", func(t *testing.T) {
		got, _, ok := resolveRoutePath(routes, "/legacy")
		if !ok || got != "/legacy/" {
			t.Fatalf("resolveRoutePath(/legacy) = (%q, %v), want (/legacy/ , true)", got, ok)
		}
	})
}
