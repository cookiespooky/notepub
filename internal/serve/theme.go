package serve

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cookiespooky/notepub/internal/models"
)

//go:embed embed/templates/*.html
//go:embed embed/assets/*
var embeddedFS embed.FS

type Theme struct {
	layoutName   string
	pageName     string
	layout       *template.Template
	page         *template.Template
	assetsDir    string
	assetsSubdir string
	assetFS      fs.FS
	fallbackFS   fs.FS
	hasHomeCSS   bool
	usedFallback bool
}

type PageData struct {
	Title            string
	Canonical        string
	Meta             MetaData
	Body             template.HTML
	Template         string
	Error            string
	IsHome           bool
	IsCategory       bool
	IsSearch         bool
	HasHomeCSS       bool
	Catalog          *models.Catalog
	CatalogJSON      string
	Page             PageInfo
	Core             CoreFields
	FM               map[string]interface{}
	Collections      map[string]models.CollectionResult
	Category         *models.CategoryModel
	CategoryItems    []models.CatalogItem
	SearchQuery      string
	SearchItems      []SearchItem
	SearchNextCursor string
	SearchMode       string
}

type PageInfo struct {
	Type        string
	Slug        string
	Title       string
	Description string
	Canonical   string
	Category    *models.CategoryModel
	NoIndex     bool
}

type CoreFields struct {
	Type        string
	Slug        string
	Title       string
	Description string
}

type MetaData struct {
	Robots    string
	OpenGraph []MetaKV
	JSONLD    string
}

type MetaKV struct {
	Key   string
	Value string
}

func LoadTheme(themeDir, templatesSubdir, assetsSubdir string) (*Theme, error) {
	fallbackFS, err := fs.Sub(embeddedFS, "embed")
	if err != nil {
		return nil, err
	}

	if templatesSubdir == "" {
		templatesSubdir = "templates"
	}
	if assetsSubdir == "" {
		assetsSubdir = "assets"
	}
	assetsDir := filepath.Join(themeDir, assetsSubdir)
	templatesDir := filepath.Join(themeDir, templatesSubdir)
	layoutPath := filepath.Join(templatesDir, "layout.html")
	pagePath := filepath.Join(templatesDir, "page.html")

	layoutExists := fileExists(layoutPath)
	pageExists := fileExists(pagePath)

	if layoutExists || pageExists {
		tmpl, err := parseTemplatesDir(templatesDir)
		if err == nil {
			hasHomeCSS := fileExists(filepath.Join(assetsDir, "home.css"))
			return &Theme{
				layoutName:   "layout.html",
				pageName:     "page.html",
				layout:       tmpl,
				page:         tmpl,
				assetsDir:    assetsDir,
				assetsSubdir: assetsSubdir,
				assetFS:      os.DirFS(themeDir),
				fallbackFS:   fallbackFS,
				hasHomeCSS:   hasHomeCSS,
				usedFallback: false,
			}, nil
		}
	}

	fallbackTemplates, err := parseTemplatesFS(fallbackFS, "templates")
	if err != nil {
		return nil, err
	}
	_, err = fs.Stat(fallbackFS, filepath.ToSlash(filepath.Join("assets", "home.css")))
	hasHomeCSS := err == nil

	return &Theme{
		layoutName:   "layout.html",
		pageName:     "page.html",
		layout:       fallbackTemplates,
		page:         fallbackTemplates,
		assetsSubdir: "assets",
		assetFS:      fallbackFS,
		fallbackFS:   fallbackFS,
		hasHomeCSS:   hasHomeCSS,
		usedFallback: true,
	}, nil
}

func (t *Theme) UsedFallback() bool {
	return t.usedFallback
}

func (t *Theme) RenderPage(data PageData) (string, error) {
	body := data.Body
	bodyTemplate := t.pageName
	if data.Template != "" && templateExists(t.page, data.Template) {
		bodyTemplate = data.Template
	} else {
		if data.IsHome && templateExists(t.page, "home.html") {
			bodyTemplate = "home.html"
		}
		if data.IsCategory && templateExists(t.page, "category.html") {
			bodyTemplate = "category.html"
		}
		if data.IsSearch && templateExists(t.page, "search.html") {
			bodyTemplate = "search.html"
		}
	}
	data.HasHomeCSS = t.hasHomeCSS
	if t.page != nil && templateExists(t.page, bodyTemplate) {
		var buf bytes.Buffer
		if err := t.page.ExecuteTemplate(&buf, bodyTemplate, data); err != nil {
			return "", err
		}
		body = template.HTML(buf.String())
	}
	data.Body = body

	if t.layout != nil && templateExists(t.layout, t.layoutName) {
		var buf bytes.Buffer
		if err := t.layout.ExecuteTemplate(&buf, t.layoutName, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	return string(body), nil
}

func (t *Theme) RenderError(err error, data PageData) (string, error) {
	data.Error = err.Error()
	if t.layout != nil && templateExists(t.layout, "error.html") {
		var buf bytes.Buffer
		if execErr := t.layout.ExecuteTemplate(&buf, "error.html", data); execErr == nil {
			return buf.String(), nil
		}
	}
	if t.layout != nil && templateExists(t.layout, t.layoutName) {
		data.Body = template.HTML(fmt.Sprintf("<section class=\"section\"><h1 class=\"section-title\">Render error</h1><pre>%s</pre></section>", template.HTMLEscapeString(err.Error())))
		var buf bytes.Buffer
		if execErr := t.layout.ExecuteTemplate(&buf, t.layoutName, data); execErr == nil {
			return buf.String(), nil
		}
	}
	return fmt.Sprintf("render error: %s", err.Error()), nil
}

func (t *Theme) RenderNotFound() (string, error) {
	if t.layout != nil && templateExists(t.layout, "notfound.html") {
		var buf bytes.Buffer
		if err := t.layout.ExecuteTemplate(&buf, "notfound.html", nil); err == nil {
			return buf.String(), nil
		}
	}
	return "Not Found", nil
}

func (t *Theme) AssetFS() fs.FS {
	if t.assetFS != nil {
		return t.assetFS
	}
	return t.fallbackFS
}

func parseTemplatesDir(dir string) (*template.Template, error) {
	files, err := collectTemplateFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no templates found in %s", dir)
	}
	return template.ParseFiles(files...)
}

func parseTemplatesFS(fsRoot fs.FS, subdir string) (*template.Template, error) {
	fsDir, err := fs.Sub(fsRoot, subdir)
	if err != nil {
		return nil, err
	}
	return template.ParseFS(fsDir, "*.html", "partials/*.html")
}

func templateExists(t *template.Template, name string) bool {
	if t == nil {
		return false
	}
	return t.Lookup(name) != nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func collectTemplateFiles(dir string) ([]string, error) {
	root := filepath.Join(dir, "*.html")
	partials := filepath.Join(dir, "partials", "*.html")
	rootFiles, err := filepath.Glob(root)
	if err != nil {
		return nil, err
	}
	partialFiles, err := filepath.Glob(partials)
	if err != nil {
		return nil, err
	}
	return append(rootFiles, partialFiles...), nil
}
