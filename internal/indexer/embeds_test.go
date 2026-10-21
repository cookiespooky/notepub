package indexer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbedsAreFoundByNameAcrossTheVault(t *testing.T) {
	site := newSEOSite(t, map[string]string{
		"home.md":          "---\ntype: home\nslug: home\ntitle: Главная\n---\n",
		"raboty/komod.md":  "---\ntype: post\nslug: komod\ntitle: Комод\n---\n![[komod.svg|Комод]]\n\n![[logo.svg]]\n",
		"raboty/broken.md": "---\ntype: post\nslug: broken\ntitle: Нет файла\n---\n![[nope.png]]\n",
		"foto/komod.svg":   "<svg/>",
	})
	mediaDir := filepath.Join(site.root, "media")
	os.MkdirAll(mediaDir, 0o755)
	os.WriteFile(filepath.Join(mediaDir, "logo.svg"), []byte("<svg/>"), 0o644)
	site.cfg.Media.Dirs = []string{mediaDir}

	idx := site.index(t)
	if got := idx.Meta["/komod"].Image; got != "https://example.com/media/foto/komod.svg" {
		t.Errorf("картинка для соцсетей должна найтись по имени в foto/, получено %q", got)
	}
	if got := strings.Join(idx.Media["/komod"], ","); got != "foto/komod.svg,logo.svg" {
		t.Errorf("медиа страницы: %q", got)
	}
	if got := strings.Join(idx.MediaFiles, ","); got != "foto/komod.svg,logo.svg" {
		t.Errorf("список файлов: %q", got)
	}

	diags, _, err := ValidateMarkdownWithCapabilities(context.Background(), site.cfg, idx)
	if err != nil {
		t.Fatal(err)
	}
	missing := 0
	for _, d := range diags {
		if d.Code == "NP-MD-EMBED-MISSING" {
			missing++
			if !strings.Contains(d.File, "broken") {
				t.Errorf("лишнее предупреждение: %+v", d)
			}
		}
	}
	if missing != 1 {
		t.Errorf("ожидалось одно предупреждение о nope.png, получено %d: %+v", missing, diags)
	}

	// A new file in the vault re-indexes notes whose embeds it may now answer.
	os.WriteFile(filepath.Join(site.cfg.Content.LocalDir, "nope.png"), []byte("png"), 0o644)
	idx = site.index(t)
	if got := idx.Meta["/broken"].Image; got != "https://example.com/media/nope.png" {
		t.Errorf("после появления файла картинка не нашлась: %q", got)
	}
}
