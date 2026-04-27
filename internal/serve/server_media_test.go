package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalMediaPathFallbackToSiblingMediaDir(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "content")
	mediaDir := filepath.Join(root, "media")
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatalf("mkdir media: %v", err)
	}
	wantPath := filepath.Join(mediaDir, "logo.svg")
	if err := os.WriteFile(wantPath, []byte("<svg></svg>"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	gotPath, err := resolveLocalMediaPath(contentDir, "logo.svg")
	if err != nil {
		t.Fatalf("resolveLocalMediaPath returned error: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("resolveLocalMediaPath = %q, want %q", gotPath, wantPath)
	}
}
