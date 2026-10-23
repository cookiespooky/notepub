// Package scaffold makes the files a new part of a site needs, so that adding
// a kind of page is one command instead of an edit in rules.yaml plus a
// template copied by hand.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var typeName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// TypeOptions describe the type to add.
type TypeOptions struct {
	Name         string
	RulesPath    string
	TemplatesDir string
	Permalink    string // default "/<name>/{{ slug }}/"
	Template     string // default "<name>.html"
	From         string // template copied for the new one; default "page.html"
	Sitemap      bool
	Search       bool
}

// TypeResult says what was written.
type TypeResult struct {
	Permalink       string
	Template        string
	TemplatePath    string
	TemplateCreated bool   // false when a template of that name already existed
	TemplateSource  string // what it was copied from, or "minimal"
}

// AddType adds a type to rules.yaml, keeping the file's comments and order,
// and creates its template from an existing one.
func AddType(opts TypeOptions) (TypeResult, error) {
	name := strings.TrimSpace(opts.Name)
	if !typeName.MatchString(name) {
		return TypeResult{}, fmt.Errorf("type name %q: use lowercase latin letters, digits, - and _ (e.g. service, case_study)", name)
	}
	permalink := strings.TrimSpace(opts.Permalink)
	if permalink == "" {
		permalink = "/" + name + "/{{ slug }}/"
	}
	if !strings.HasPrefix(permalink, "/") {
		return TypeResult{}, fmt.Errorf("permalink must start with /: %q", permalink)
	}
	tmplName := strings.TrimSpace(opts.Template)
	if tmplName == "" {
		tmplName = name + ".html"
	}
	if filepath.Base(tmplName) != tmplName || !strings.HasSuffix(tmplName, ".html") {
		return TypeResult{}, fmt.Errorf("template must be a file name ending in .html: %q", tmplName)
	}

	src, err := os.ReadFile(opts.RulesPath)
	if err != nil {
		return TypeResult{}, fmt.Errorf("read rules: %w", err)
	}
	var parsed struct {
		Types map[string]interface{} `yaml:"types"`
	}
	if err := yaml.Unmarshal(src, &parsed); err != nil {
		return TypeResult{}, fmt.Errorf("parse %s: %w", opts.RulesPath, err)
	}
	if _, exists := parsed.Types[name]; exists {
		return TypeResult{}, fmt.Errorf("type %q is already in %s", name, opts.RulesPath)
	}

	updated := insertType(string(src), name, typeBlock(permalink, tmplName, opts.Sitemap, opts.Search))
	var check struct {
		Types map[string]map[string]interface{} `yaml:"types"`
	}
	if err := yaml.Unmarshal([]byte(updated), &check); err != nil || check.Types[name] == nil {
		return TypeResult{}, fmt.Errorf("could not add the type to %s without breaking it; add it by hand", opts.RulesPath)
	}

	res := TypeResult{Permalink: permalink, Template: tmplName, TemplatePath: filepath.Join(opts.TemplatesDir, tmplName)}
	if _, err := os.Stat(res.TemplatePath); err != nil {
		from := strings.TrimSpace(opts.From)
		if from == "" {
			from = "page.html"
		}
		body, err := os.ReadFile(filepath.Join(opts.TemplatesDir, from))
		res.TemplateSource = from
		if err != nil {
			if strings.TrimSpace(opts.From) != "" {
				return TypeResult{}, fmt.Errorf("template to copy not found: %s", filepath.Join(opts.TemplatesDir, from))
			}
			body = []byte(minimalTemplate)
			res.TemplateSource = "minimal"
		}
		if err := os.MkdirAll(opts.TemplatesDir, 0o755); err != nil {
			return TypeResult{}, err
		}
		if err := os.WriteFile(res.TemplatePath, body, 0o644); err != nil {
			return TypeResult{}, err
		}
		res.TemplateCreated = true
	}
	if err := os.WriteFile(opts.RulesPath, []byte(updated), 0o644); err != nil {
		return TypeResult{}, err
	}
	return res, nil
}

const minimalTemplate = `<article class="content">
  <h1>{{ .Title }}</h1>
  {{- if .Page.Description }}
  <p class="lead">{{ .Page.Description }}</p>
  {{- end }}
  {{ .Body }}
</article>
`

func typeBlock(permalink, tmpl string, sitemap, search bool) []string {
	return []string{
		fmt.Sprintf("template: %q", tmpl),
		fmt.Sprintf("permalink: %q", permalink),
		"include_in:",
		fmt.Sprintf("  sitemap: %t", sitemap),
		fmt.Sprintf("  search: %t", search),
	}
}

var topKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*\s*:`)

// insertType writes the block at the end of the top-level types: map, with
// the indentation the file already uses, or adds types: when there is none.
func insertType(src, name string, block []string) string {
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if regexp.MustCompile(`^types\s*:\s*(#.*)?$`).MatchString(l) {
			start = i
			break
		}
	}
	if start < 0 {
		out := strings.TrimRight(src, "\n") + "\n\ntypes:\n" + render("  ", name, block)
		return out
	}
	indent := "  "
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if topKey.MatchString(l) {
			end = i
			break
		}
		if indent == "  " && i == firstContent(lines, start+1) {
			indent = l[:len(l)-len(strings.TrimLeft(l, " "))]
		}
	}
	// Step back over blank lines and comments that belong to the next key.
	last := end
	for last > start+1 && (strings.TrimSpace(lines[last-1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[last-1]), "#")) {
		last--
	}
	insert := strings.Split(strings.TrimRight(render(indent, name, block), "\n"), "\n")
	out := append([]string{}, lines[:last]...)
	if last > start+1 {
		out = append(out, "")
	}
	out = append(out, insert...)
	out = append(out, lines[last:]...)
	return strings.Join(out, "\n")
}

func firstContent(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t != "" && !strings.HasPrefix(t, "#") {
			return i
		}
	}
	return -1
}

func render(indent, name string, block []string) string {
	var b strings.Builder
	b.WriteString(indent + name + ":\n")
	for _, l := range block {
		b.WriteString(indent + indent + l + "\n")
	}
	return b.String()
}
