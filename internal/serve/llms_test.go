package serve

import (
	"strings"
	"testing"

	"github.com/cookiespooky/notepub/internal/config"
	"github.com/cookiespooky/notepub/internal/rules"
)

func TestRenderLLMSOffByDefault(t *testing.T) {
	idx, cfg := feedFixture()
	if _, ok := RenderLLMS(idx, cfg, rules.Rules{}); ok {
		t.Fatal("llms.txt must not be written unless llms.enabled is set")
	}
}

func TestRenderLLMSListsIndexedPagesBySection(t *testing.T) {
	idx, cfg := feedFixture()
	cfg.Site.Description = "Заметки о   коде"
	cfg.LLMS = config.LLMSConfig{Enabled: true, Path: "llms.txt", Sections: []config.LLMSSection{
		{Type: "post", Title: "Посты"},
		{Type: "page", Title: "Страницы"},
	}}
	out, ok := RenderLLMS(idx, cfg, rules.Rules{})
	if !ok {
		t.Fatal("llms.txt not rendered")
	}
	got := string(out)
	for _, want := range []string{
		"# Блог\n",
		"> Заметки о коде\n",
		"## Посты\n\n- [Новый](https://ex.com/new/): Свежий пост\n- [Старый](https://ex.com/old/)\n",
		"## Страницы\n\n- [Обо мне](https://ex.com/about/)\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	for _, banned := range []string{"Черновик", "Скрытый", "page/2", "home"} {
		if strings.Contains(got, banned) {
			t.Fatalf("%q must not be listed:\n%s", banned, got)
		}
	}
	if strings.Index(got, "## Посты") > strings.Index(got, "## Страницы") {
		t.Fatal("sections must keep the configured order")
	}
	again, _ := RenderLLMS(idx, cfg, rules.Rules{})
	if string(again) != got {
		t.Fatal("output must not depend on map order")
	}
}

func TestRenderLLMSDefaultsToEveryType(t *testing.T) {
	idx, cfg := feedFixture()
	cfg.LLMS = config.LLMSConfig{Enabled: true}
	out, _ := RenderLLMS(idx, cfg, rules.Rules{})
	got := string(out)
	for _, want := range []string{"## home\n", "## page\n", "## post\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing section %q:\n%s", want, got)
		}
	}
}
