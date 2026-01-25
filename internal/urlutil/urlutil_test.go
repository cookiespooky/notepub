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
