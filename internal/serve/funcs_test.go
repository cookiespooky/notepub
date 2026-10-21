package serve

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDateFuncs(t *testing.T) {
	tmpl := template.Must(template.New("t").Funcs(templateFuncs()).Parse(
		`{{ humanDate .A "ru" }}|{{ humanDate .B "en" }}|{{ isoDate .A }}|{{ humanDate .C "ru" }}|{{ isoDate .C }}`))
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, map[string]interface{}{
		"A": "2025-03-04T10:00:00Z",
		"B": time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC),
		"C": "не дата",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "4 марта 2025|October 21, 2026|2025-03-04|не дата|"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestThemeTemplatesCanCallDateFuncs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir+"/templates/layout.html", `<p>{{ humanDate .Meta.Published "ru" }}</p>{{ .Body }}`)
	writeTestFile(t, dir+"/templates/page.html", `body`)
	theme, err := LoadTheme(dir, "templates", "assets")
	if err != nil {
		t.Fatal(err)
	}
	if theme.UsedFallback() {
		t.Fatal("тема не загрузилась: функции не зарегистрированы при разборе шаблонов")
	}
	out, err := theme.RenderPage(PageData{Meta: MetaData{Published: "2025-01-02T00:00:00Z"}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "<p>2 января 2025</p>body" {
		t.Errorf("got %q", out)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDictFunc(t *testing.T) {
	tmpl := template.Must(template.New("t").Funcs(templateFuncs()).Parse(
		`{{ define "x" }}{{ .A }}-{{ .B }}{{ end }}{{ template "x" (dict "A" 1 "B" "два") }}`))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "1-два" {
		t.Errorf("got %q", buf.String())
	}
	if err := template.Must(template.New("bad").Funcs(templateFuncs()).Parse(`{{ dict "A" }}`)).Execute(&buf, nil); err == nil {
		t.Error("нечётное число аргументов должно быть ошибкой")
	}
}
