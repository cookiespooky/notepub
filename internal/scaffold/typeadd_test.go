package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const rulesSrc = `version: 1

# the kinds of pages
types:
  page:
    template: "page.html"   # plain pages
    permalink: "/{{ slug }}/"
    include_in:
      sitemap: true
      search: true

  # posts live under /blog/
  post:
    template: "post.html"
    permalink: "/blog/{{ slug }}/"

# links between notes
links: []
`

func setup(t *testing.T, rules string) (string, string) {
	dir := t.TempDir()
	rp := filepath.Join(dir, "rules.yaml")
	os.WriteFile(rp, []byte(rules), 0o644)
	td := filepath.Join(dir, "theme", "templates")
	os.MkdirAll(td, 0o755)
	os.WriteFile(filepath.Join(td, "page.html"), []byte("<article>{{ .Body }}</article>\n"), 0o644)
	return rp, td
}

func TestAddTypeKeepsCommentsAndOrder(t *testing.T) {
	rp, td := setup(t, rulesSrc)
	res, err := AddType(TypeOptions{Name: "service", RulesPath: rp, TemplatesDir: td, Sitemap: true, Search: true})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(rp)
	got := string(out)
	for _, keep := range []string{"# the kinds of pages", "# plain pages", "# posts live under /blog/", "# links between notes"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("lost comment %q:\n%s", keep, got)
		}
	}
	if strings.Index(got, "  service:") < strings.Index(got, "  post:") || strings.Index(got, "  service:") > strings.Index(got, "# links between notes") {
		t.Fatalf("the type must go at the end of types:, before the next key and its comment:\n%s", got)
	}
	var r struct {
		Types map[string]struct {
			Template  string `yaml:"template"`
			Permalink string `yaml:"permalink"`
			IncludeIn struct {
				Sitemap bool `yaml:"sitemap"`
				Search  bool `yaml:"search"`
			} `yaml:"include_in"`
		} `yaml:"types"`
		Links []interface{} `yaml:"links"`
	}
	if err := yaml.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	s := r.Types["service"]
	if s.Template != "service.html" || s.Permalink != "/service/{{ slug }}/" || !s.IncludeIn.Sitemap || !s.IncludeIn.Search {
		t.Fatalf("wrong type: %+v", s)
	}
	if len(r.Types) != 3 {
		t.Fatalf("other types changed: %d", len(r.Types))
	}
	body, err := os.ReadFile(res.TemplatePath)
	if err != nil || string(body) != "<article>{{ .Body }}</article>\n" || !res.TemplateCreated || res.TemplateSource != "page.html" {
		t.Fatalf("template not copied from page.html: %q %+v", body, res)
	}
}

func TestAddTypeRefusesExistingAndBadNames(t *testing.T) {
	rp, td := setup(t, rulesSrc)
	if _, err := AddType(TypeOptions{Name: "post", RulesPath: rp, TemplatesDir: td}); err == nil {
		t.Fatal("an existing type must be refused")
	}
	for _, bad := range []string{"", "Услуга", "My Type", "../x"} {
		if _, err := AddType(TypeOptions{Name: bad, RulesPath: rp, TemplatesDir: td}); err == nil {
			t.Fatalf("name %q must be refused", bad)
		}
	}
	after, _ := os.ReadFile(rp)
	if string(after) != rulesSrc {
		t.Fatal("a refused call must not touch rules.yaml")
	}
}

func TestAddTypeWithoutTypesSectionAndKeepsExistingTemplate(t *testing.T) {
	rp, td := setup(t, "version: 1\nlinks: []\n")
	os.WriteFile(filepath.Join(td, "case.html"), []byte("mine"), 0o644)
	res, err := AddType(TypeOptions{Name: "case", RulesPath: rp, TemplatesDir: td, Permalink: "/cases/{{ slug }}/"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TemplateCreated {
		t.Fatal("an existing template must not be overwritten")
	}
	body, _ := os.ReadFile(filepath.Join(td, "case.html"))
	if string(body) != "mine" {
		t.Fatal("template overwritten")
	}
	out, _ := os.ReadFile(rp)
	if !strings.Contains(string(out), "types:\n  case:\n    template: \"case.html\"\n    permalink: \"/cases/{{ slug }}/\"") {
		t.Fatalf("types: not added:\n%s", out)
	}
}

func TestAddTypeFourSpaceIndent(t *testing.T) {
	rp, td := setup(t, "types:\n    page:\n        template: \"page.html\"\n        permalink: \"/{{ slug }}/\"\n")
	if _, err := AddType(TypeOptions{Name: "faq", RulesPath: rp, TemplatesDir: td}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(rp)
	if !strings.Contains(string(out), "\n    faq:\n        template: \"faq.html\"") {
		t.Fatalf("indent of the file not kept:\n%s", out)
	}
}
