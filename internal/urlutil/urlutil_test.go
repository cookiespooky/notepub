package urlutil

import "testing"

func TestJoinBaseURL(t *testing.T) {
	tests := []struct {
		base string
		path string
		want string
	}{
		{"https://user.github.io/repo", "/slug", "https://user.github.io/repo/slug"},
		{"https://user.github.io/repo/", "/slug/", "https://user.github.io/repo/slug/"},
		{"https://user.github.io/repo", "/", "https://user.github.io/repo/"},
		{"https://user.github.io/repo", "", "https://user.github.io/repo/"},
		{"https://example.com", "/slug", "https://example.com/slug"},
		{"https://example.com/", "/", "https://example.com/"},
	}
	for _, tt := range tests {
		got := JoinBaseURL(tt.base, tt.path)
		if got != tt.want {
			t.Fatalf("JoinBaseURL(%q, %q) = %q, want %q", tt.base, tt.path, got, tt.want)
		}
	}
}

func TestPublicPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/", "/"},
		{"", "/"},
		{"/cases/notepub", "/cases/notepub/"},
		{"/cases/notepub/", "/cases/notepub/"},
		{"cases/notepub", "/cases/notepub/"},
		{"/blog", "/blog/"},
	}
	for _, tt := range tests {
		if got := PublicPath(tt.path); got != tt.want {
			t.Fatalf("PublicPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// The builder writes every route as <path>/index.html, so a canonical link
// built from a route key has to carry the trailing slash — otherwise it points
// at an address the host only redirects from.
func TestPublicPathFeedsCanonicalWithTrailingSlash(t *testing.T) {
	got := JoinBaseURL("https://example.com", PublicPath("/cases/notepub"))
	if want := "https://example.com/cases/notepub/"; got != want {
		t.Fatalf("canonical = %q, want %q", got, want)
	}
	if got := JoinBaseURL("https://example.com", PublicPath("/")); got != "https://example.com/" {
		t.Fatalf("root canonical = %q, want %q", got, "https://example.com/")
	}
}
