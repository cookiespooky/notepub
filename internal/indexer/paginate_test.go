package indexer

import (
	"testing"

	"github.com/cookiespooky/notepub/internal/models"
	"github.com/cookiespooky/notepub/internal/rules"
)

func paginationFixture(articles int) (models.ResolveIndex, rules.Rules) {
	idx := models.ResolveIndex{
		Routes: map[string]models.RouteEntry{
			"/blog": {S3Key: "content/blog.md", Status: 200},
		},
		Meta: map[string]models.MetaEntry{
			"/blog": {Type: "blog", Slug: "blog", Title: "Блог"},
		},
	}
	for i := 0; i < articles; i++ {
		p := "/blog/post-" + string(rune('a'+i))
		idx.Routes[p] = models.RouteEntry{S3Key: "content/blog/post.md", Status: 200}
		idx.Meta[p] = models.MetaEntry{
			Type: "article",
			Slug: "post-" + string(rune('a'+i)),
			FM:   map[string]interface{}{"draft": false},
		}
	}
	cfg := rules.Rules{
		Types: map[string]rules.TypeDef{
			"blog": {
				Template:  "blog.html",
				Permalink: "/blog/",
				Paginate:  &rules.PaginateRule{Collection: "posts_all", PerPage: 10, Path: "/blog/page/{{ n }}/"},
			},
			"article": {Template: "article.html", Permalink: "/blog/{{ slug }}/"},
		},
		Collections: map[string]rules.CollectionRule{
			"posts_all": {
				Kind: "filter",
				Where: rules.WhereRule{All: []map[string]interface{}{
					{"type_in": []interface{}{"article"}},
				}},
				Limit: 500,
			},
		},
	}
	return idx, cfg
}

func TestSynthesizePaginatedRoutes(t *testing.T) {
	idx, cfg := paginationFixture(14)
	used := map[string]bool{"/blog": true}

	if err := synthesizePaginatedRoutes(&idx, cfg, "https://example.com", used); err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	route, ok := idx.Routes["/blog/page/2"]
	if !ok {
		t.Fatal("маршрут второй страницы не создан")
	}
	if route.PageNum != 2 {
		t.Errorf("PageNum = %d, want 2", route.PageNum)
	}
	if route.PaginateOf != "/blog" {
		t.Errorf("PaginateOf = %q, want %q", route.PaginateOf, "/blog")
	}
	if route.S3Key != "content/blog.md" {
		t.Errorf("страница должна рендериться из того же файла, получено %q", route.S3Key)
	}
	if _, ok := idx.Routes["/blog/page/3"]; ok {
		t.Error("создана лишняя третья страница: 14 статей по 10 — это две страницы")
	}
	if _, ok := idx.Routes["/blog/page/1"]; ok {
		t.Error("первая страница должна оставаться на /blog, отдельный маршрут не нужен")
	}

	meta := idx.Meta["/blog/page/2"]
	if meta.Slug != "" {
		t.Errorf("Slug синтетической страницы = %q, должен быть пустым: иначе слаг резолвится в два пути", meta.Slug)
	}
	if meta.Type != "blog" {
		t.Errorf("Type = %q, want blog: по нему выбирается шаблон", meta.Type)
	}
	if want := "https://example.com/blog/page/2/"; meta.Canonical != want {
		t.Errorf("Canonical = %q, want %q", meta.Canonical, want)
	}
}

// Синтетика не должна попадать в слаг-индекс: иначе слаг «blog» резолвится то в
// /blog, то в /blog/page/2 — порядок обхода map в Go рандомизирован.
func TestPaginatedRouteStaysOutOfSlugIndex(t *testing.T) {
	idx, cfg := paginationFixture(14)
	if err := synthesizePaginatedRoutes(&idx, cfg, "https://example.com", map[string]bool{"/blog": true}); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	for i := 0; i < 50; i++ {
		if got := buildSlugIndex(idx)["blog"]; got != "/blog" {
			t.Fatalf("слаг blog разрешился в %q, ожидался /blog", got)
		}
	}
}

func TestSynthesizeSkipsWhenOnePage(t *testing.T) {
	idx, cfg := paginationFixture(4)
	if err := synthesizePaginatedRoutes(&idx, cfg, "https://example.com", map[string]bool{"/blog": true}); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if _, ok := idx.Routes["/blog/page/2"]; ok {
		t.Error("четыре статьи по десять — вторая страница не нужна")
	}
}

func TestSynthesizeRejectsCollision(t *testing.T) {
	idx, cfg := paginationFixture(14)
	used := map[string]bool{"/blog": true, "/blog/page/2": true}
	err := synthesizePaginatedRoutes(&idx, cfg, "https://example.com", used)
	if err == nil {
		t.Fatal("столкновение с существующей страницей должно быть ошибкой, а не тихой перезаписью")
	}
}

func TestValidatePaginateRule(t *testing.T) {
	cfg := rules.Rules{Collections: map[string]rules.CollectionRule{"posts_all": {Kind: "filter"}}}
	bad := []rules.PaginateRule{
		{Collection: "", PerPage: 10, Path: "/p/{{ n }}/"},
		{Collection: "нет такой", PerPage: 10, Path: "/p/{{ n }}/"},
		{Collection: "posts_all", PerPage: 0, Path: "/p/{{ n }}/"},
		{Collection: "posts_all", PerPage: 10, Path: ""},
		{Collection: "posts_all", PerPage: 10, Path: "/p/2/"},
	}
	for i, rule := range bad {
		if err := validatePaginateRule("blog", rule, cfg); err == nil {
			t.Errorf("правило %d должно быть отвергнуто", i)
		}
	}
	good := rules.PaginateRule{Collection: "posts_all", PerPage: 10, Path: "/blog/page/{{ n }}/"}
	if err := validatePaginateRule("blog", good, cfg); err != nil {
		t.Errorf("корректное правило отвергнуто: %v", err)
	}
}
