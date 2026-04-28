package templateupdate

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type UpdateOptions struct {
	Root  string
	Apply bool
}

type projectLayout string

const (
	layoutUnknown projectLayout = "unknown"
	layoutLegacy  projectLayout = "legacy"
	layoutModern  projectLayout = "modern"
)

var (
	rootAbsoluteRefRe = regexp.MustCompile(`(?:href|src)=["'](/(?:assets|media|[A-Za-z0-9._~-]+/)[^"']*)["']`)
	titleLineRe       = regexp.MustCompile(`(?m)^title:\s*["']?([^"'\n]+)["']?\s*$`)
)

func Check(root string) (string, error) {
	root, err := cleanRoot(root)
	if err != nil {
		return "", err
	}
	layout := detectLayout(root)
	if layout == layoutUnknown {
		return "", fmt.Errorf("not a Notepub template project: %s", root)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Template project: %s\n", root)
	fmt.Fprintf(&b, "Layout: %s\n", layout)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Checks:")
	for _, item := range checks(root, layout) {
		fmt.Fprintf(&b, "- %s\n", item)
	}

	manual := manualFindings(root)
	if len(manual) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Manual review:")
		for _, item := range manual {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	return b.String(), nil
}

func Update(opts UpdateOptions) (string, error) {
	root, err := cleanRoot(opts.Root)
	if err != nil {
		return "", err
	}
	layout := detectLayout(root)
	if layout == layoutUnknown {
		return "", fmt.Errorf("not a Notepub template project: %s", root)
	}

	changes, err := plannedChanges(root, layout)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Template project: %s\n", root)
	fmt.Fprintf(&b, "Layout: %s\n", layout)
	if opts.Apply {
		fmt.Fprintln(&b, "Mode: apply")
	} else {
		fmt.Fprintln(&b, "Mode: dry-run")
	}
	fmt.Fprintln(&b)

	if len(changes) == 0 {
		fmt.Fprintln(&b, "No infrastructure changes needed.")
	} else {
		fmt.Fprintln(&b, "Planned changes:")
		for _, ch := range changes {
			fmt.Fprintf(&b, "- %s\n", rel(root, ch.path))
		}
	}

	if opts.Apply && len(changes) > 0 {
		if err := writeBackup(root, changes); err != nil {
			return "", err
		}
		for _, ch := range changes {
			if err := os.MkdirAll(filepath.Dir(ch.path), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(ch.path, []byte(ch.next), ch.mode); err != nil {
				return "", fmt.Errorf("write %s: %w", ch.path, err)
			}
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Updated template infrastructure.")
	} else if !opts.Apply {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Dry run only. Re-run with --apply to write changes.")
	}

	manual := manualFindings(root)
	if len(manual) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Manual review:")
		for _, item := range manual {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	return b.String(), nil
}

type fileChange struct {
	path string
	prev string
	next string
	mode fs.FileMode
}

func plannedChanges(root string, layout projectLayout) ([]fileChange, error) {
	switch layout {
	case layoutLegacy:
		return plannedLegacyChanges(root)
	case layoutModern:
		return plannedModernChanges(root)
	default:
		return nil, fmt.Errorf("unsupported template layout: %s", layout)
	}
}

func plannedLegacyChanges(root string) ([]fileChange, error) {
	workflowPath := filepath.Join(root, ".github", "workflows", "deploy.yml")
	plans := []struct {
		path string
		next string
		mode fs.FileMode
	}{
		{filepath.Join(root, "scripts", "build.sh"), legacyBuildScript(), 0o755},
		{workflowPath, legacyDeployWorkflow(workflowName(workflowPath), workflowNotepubVersion(workflowPath)), 0o644},
	}
	var changes []fileChange
	for _, p := range plans {
		ch, err := changeIfDifferent(p.path, p.next, p.mode)
		if err != nil {
			return nil, err
		}
		if ch != nil {
			changes = append(changes, *ch)
		}
	}
	if ch, err := patchConfig(filepath.Join(root, "config.yaml"), false); err != nil {
		return nil, err
	} else if ch != nil {
		changes = append(changes, *ch)
	}
	if ch, err := patchGitignore(filepath.Join(root, ".gitignore"), "config.resolved.yaml"); err != nil {
		return nil, err
	} else if ch != nil {
		changes = append(changes, *ch)
	}
	if noteChanges, err := rootNoteChanges(root, filepath.Join(root, "config.yaml")); err != nil {
		return nil, err
	} else {
		changes = append(changes, noteChanges...)
	}
	return changes, nil
}

func plannedModernChanges(root string) ([]fileChange, error) {
	var changes []fileChange
	workflowPath := filepath.Join(root, ".github", "workflows", "deploy.yml")
	plans := []struct {
		path string
		next string
		mode fs.FileMode
	}{
		{filepath.Join(root, ".np", "scripts", "build.sh"), modernBuildScript(), 0o755},
		{workflowPath, modernDeployWorkflow(workflowName(workflowPath), workflowNotepubVersion(workflowPath)), 0o644},
	}
	for _, p := range plans {
		ch, err := changeIfDifferent(p.path, p.next, p.mode)
		if err != nil {
			return nil, err
		}
		if ch != nil {
			changes = append(changes, *ch)
		}
	}
	if ch, err := patchConfig(filepath.Join(root, ".np", "config.yaml"), true); err != nil {
		return nil, err
	} else if ch != nil {
		changes = append(changes, *ch)
	}
	if ch, err := patchGitignore(filepath.Join(root, ".gitignore"), ".np/config.resolved.yaml"); err != nil {
		return nil, err
	} else if ch != nil {
		changes = append(changes, *ch)
	}
	if noteChanges, err := rootNoteChanges(root, filepath.Join(root, ".np", "config.yaml")); err != nil {
		return nil, err
	} else {
		changes = append(changes, noteChanges...)
	}
	return changes, nil
}

func changeIfDifferent(path, next string, mode fs.FileMode) (*fileChange, error) {
	prevBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	prev := string(prevBytes)
	if prev == next {
		return nil, nil
	}
	if mode == 0 {
		mode = 0o644
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
	}
	return &fileChange{path: path, prev: prev, next: next, mode: mode}, nil
}

func patchConfig(path string, modern bool) (*fileChange, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	prev := string(data)
	next := prev
	if !strings.Contains(next, "media_base_url:") {
		next = replaceFirstLine(next, regexp.MustCompile(`(?m)^(\s*)base_url:\s*.*$`), func(line string, m []string) string {
			indent := m[1]
			return line + "\n" + indent + `media_base_url: "http://127.0.0.1:8080/media/" # local fallback; CI injects the deploy media URL automatically`
		})
	}
	if !strings.Contains(next, "\nruntime:") && !strings.HasPrefix(next, "runtime:") {
		block := `runtime:
  # auto keeps local builds on 127.0.0.1 and lets CI inject production URLs.
  mode: "auto"
`
		next = strings.Replace(next, "\ncontent:\n", "\n"+block+"content:\n", 1)
	}
	if !strings.Contains(next, "\noverrides:") && !strings.HasPrefix(next, "overrides:") {
		siteNote := "./Site.md"
		interfaceNote := "./Interface.md"
		if modern {
			siteNote = "../Site.md"
			interfaceNote = "../Interface.md"
		}
		block := fmt.Sprintf(`overrides:
  site_note: "%s"
  interface_note: "%s"
`, siteNote, interfaceNote)
		next = strings.Replace(next, "\ncontent:\n", "\n"+block+"content:\n", 1)
	}
	if !strings.Contains(next, "\nsettings:") && !strings.HasPrefix(next, "settings:") {
		block := defaultSettingsBlock(next)
		next = strings.Replace(next, "\ncontent:\n", "\n"+block+"content:\n", 1)
	}
	if modern {
		next = strings.ReplaceAll(next, "deploy media URL", "GitHub Pages media URL")
	}
	if next == prev {
		return nil, nil
	}
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	return &fileChange{path: path, prev: prev, next: next, mode: mode}, nil
}

func defaultSettingsBlock(configText string) string {
	title := yamlValue(configText, "title")
	if title == "" {
		title = "Notepub Site"
	}
	description := yamlValue(configText, "description")
	if description == "" {
		description = "Website powered by Notepub."
	}
	ogImage := yamlValue(configText, "default_og_image")
	if ogImage == "" {
		ogImage = "/media/notepub.jpg"
	}
	return fmt.Sprintf(`settings:
  site_title: %s
  site_description: %s
  site_language: "en"
  site_default_og_image: %s
`, quoteYAML(title), quoteYAML(description), quoteYAML(ogImage))
}

func rootNoteChanges(root, configPath string) ([]fileChange, error) {
	cfg := ""
	if data, err := os.ReadFile(configPath); err == nil {
		cfg = string(data)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	var changes []fileChange
	sitePath := filepath.Join(root, "Site.md")
	if !exists(sitePath) {
		title := yamlValue(cfg, "title")
		if title == "" {
			title = "Notepub Site"
		}
		description := yamlValue(cfg, "description")
		if description == "" {
			description = "Website powered by Notepub."
		}
		ogImage := yamlValue(cfg, "default_og_image")
		if ogImage == "" || ogImage == "/assets/notepub.jpg" {
			ogImage = "/media/notepub.jpg"
		}
		next := fmt.Sprintf(`---
site_title: %s
site_description: %s
site_language: en
site_default_og_image: %s
brand_name: %s
brand_logo: /media/notepub.svg
theme_accent: "#0a0a0a"
theme_link: "#0a0a0a"
theme_font: system
theme_heading_font: system
theme_radius: 14
---

# Site

Edit these properties in Obsidian to customize the site without changing config.yaml.
`, quoteYAML(title), quoteYAML(description), quoteYAML(ogImage), quoteYAML(title))
		changes = append(changes, fileChange{path: sitePath, next: next, mode: 0o644})
	}
	interfacePath := filepath.Join(root, "Interface.md")
	if !exists(interfacePath) {
		changes = append(changes, fileChange{path: interfacePath, next: defaultInterfaceNote(), mode: 0o644})
	}
	return changes, nil
}

func yamlValue(text, key string) string {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `:\s*(.+?)\s*$`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(m[1]), `"'`)
}

func quoteYAML(v string) string {
	return strconv.Quote(v)
}

func defaultInterfaceNote() string {
	return `---
ui_home: Home
ui_documentation_navigation: Documentation navigation
ui_breadcrumb: Breadcrumb
ui_open_navigation: Open navigation
ui_close_navigation: Close navigation
ui_search: Search
ui_close: Close
ui_search_placeholder: Search documentation
ui_all_results: All results
ui_search_query: Query
ui_search_no_results: No results
ui_search_no_results_found: No results found.
ui_search_loading: Loading...
ui_search_error: Error loading results
ui_next_page: Next page
ui_home_hubs_title: Documentation Hubs
ui_hub_materials_title: Section Materials
ui_related_materials_title: Related Materials
ui_not_found_title: Page not found
ui_not_found_lead: The page no longer exists or was never created.
ui_not_found_back: Back to Home
ui_error_title: Error
ui_error_lead: The template could not render this page. Reason is below.
---

# Interface

Edit these properties in Obsidian to customize basic interface labels.
`
}

func patchGitignore(path, entry string) (*fileChange, error) {
	prevBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	prev := string(prevBytes)
	lines := strings.Split(prev, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			return nil, nil
		}
	}
	next := strings.TrimRight(prev, "\n")
	if next != "" {
		next += "\n"
	}
	next += entry + "\n"
	return &fileChange{path: path, prev: prev, next: next, mode: 0o644}, nil
}

func replaceFirstLine(s string, re *regexp.Regexp, repl func(string, []string) string) string {
	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		return s
	}
	line := s[loc[0]:loc[1]]
	matches := re.FindStringSubmatch(line)
	return s[:loc[0]] + repl(line, matches) + s[loc[1]:]
}

func checks(root string, layout projectLayout) []string {
	items := []string{}
	configPath := filepath.Join(root, "config.yaml")
	buildPath := filepath.Join(root, "scripts", "build.sh")
	workflowPath := filepath.Join(root, ".github", "workflows", "deploy.yml")
	if layout == layoutModern {
		configPath = filepath.Join(root, ".np", "config.yaml")
		buildPath = filepath.Join(root, ".np", "scripts", "build.sh")
	}
	if fileContains(configPath, "media_base_url:") {
		items = append(items, "site.media_base_url is present")
	} else {
		items = append(items, "site.media_base_url is missing")
	}
	if fileContains(configPath, "\nruntime:") || fileContains(configPath, "runtime:\n") {
		items = append(items, "runtime mode config is present")
	} else {
		items = append(items, "runtime mode config is missing")
	}
	overridesPresent := fileContains(configPath, "\noverrides:") || fileContains(configPath, "overrides:\n")
	settingsPresent := fileContains(configPath, "\nsettings:") || fileContains(configPath, "settings:\n")
	if overridesPresent {
		items = append(items, "Obsidian settings note overrides are present (optional layer)")
	} else {
		items = append(items, "Obsidian settings note overrides are missing (optional)")
	}
	if settingsPresent {
		items = append(items, "settings fallback is present")
	} else {
		items = append(items, "settings fallback is missing")
	}
	if exists(filepath.Join(root, "Site.md")) && exists(filepath.Join(root, "Interface.md")) {
		items = append(items, "root settings notes are present (optional)")
	} else {
		items = append(items, "root settings notes are missing (optional)")
	}
	if fileContains(buildPath, "config.resolved.yaml") {
		items = append(items, "build script generates resolved production config")
	} else {
		items = append(items, "build script does not generate resolved production config")
	}
	if fileContains(workflowPath, "GITHUB_PAGES_BASE_URL") {
		items = append(items, "GitHub Pages base URL is passed to build")
	} else {
		items = append(items, "GitHub Pages base URL is not passed to build")
	}
	return items
}

func manualFindings(root string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range append(rootAbsoluteFindings(root), duplicateTitleFindings(root)...) {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	if len(out) > 30 {
		out = append(out[:30], fmt.Sprintf("and %d more manual findings", len(out)-30))
	}
	return out
}

func rootAbsoluteFindings(root string) []string {
	var findings []string
	for _, dir := range []string{"theme/templates", ".np/theme/templates", "theme/assets", ".np/theme/assets"} {
		base := filepath.Join(root, filepath.FromSlash(dir))
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".html" && ext != ".js" && ext != ".css" {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			matches := rootAbsoluteRefRe.FindAllSubmatch(data, -1)
			for _, m := range matches {
				findings = append(findings, fmt.Sprintf("%s uses root-absolute %s", rel(root, path), string(m[1])))
			}
			return nil
		})
	}
	return findings
}

func duplicateTitleFindings(root string) []string {
	contentDir := filepath.Join(root, "content")
	byTitle := map[string][]string{}
	_ = filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		m := titleLineRe.FindSubmatch(data)
		if len(m) < 2 {
			return nil
		}
		title := strings.TrimSpace(string(m[1]))
		if title != "" {
			byTitle[strings.ToLower(title)] = append(byTitle[strings.ToLower(title)], rel(root, path))
		}
		return nil
	})
	var findings []string
	for _, paths := range byTitle {
		if len(paths) > 1 {
			sort.Strings(paths)
			findings = append(findings, fmt.Sprintf("duplicate markdown title may cause wikimap collision: %s", strings.Join(paths, ", ")))
		}
	}
	return findings
}

func writeBackup(root string, changes []fileChange) error {
	dir := filepath.Join(root, ".notepub", "update-backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "template-update-"+backupStamp()+".txt")
	var b strings.Builder
	for _, ch := range changes {
		fmt.Fprintf(&b, "### %s\n", rel(root, ch.path))
		b.WriteString(ch.prev)
		if !strings.HasSuffix(ch.prev, "\n") {
			b.WriteString("\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func backupStamp() string {
	return time.Now().UTC().Format("20060102-150405")
}

func detectLayout(root string) projectLayout {
	if exists(filepath.Join(root, ".np", "config.yaml")) && exists(filepath.Join(root, ".np", "scripts", "build.sh")) {
		return layoutModern
	}
	if exists(filepath.Join(root, "config.yaml")) && exists(filepath.Join(root, "scripts", "build.sh")) && exists(filepath.Join(root, "theme")) {
		return layoutLegacy
	}
	return layoutUnknown
}

func cleanRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root is not a directory: %s", abs)
	}
	return abs, nil
}

func fileContains(path, needle string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), needle)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

func workflowName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "Deploy Notepub Site to GitHub Pages"
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "name:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			if name != "" {
				return strings.Trim(name, `"'`)
			}
		}
	}
	return "Deploy Notepub Site to GitHub Pages"
}

func workflowNotepubVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "v0.1.3"
	}
	re := regexp.MustCompile(`(?m)^\s*NOTEPUB_VERSION:\s*([^\s#]+)`)
	m := re.FindSubmatch(data)
	if len(m) == 2 {
		return strings.Trim(string(m[1]), `"'`)
	}
	return "v0.1.3"
}

func legacyDeployWorkflow(name, version string) string {
	if strings.TrimSpace(name) == "" {
		name = "Deploy Notepub Site to GitHub Pages"
	}
	if strings.TrimSpace(version) == "" {
		version = "v0.1.3"
	}
	return `name: ` + name + `

on:
  push:
    branches: [main]
  workflow_dispatch:
  repository_dispatch:
    types: [content-updated]

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    runs-on: ubuntu-latest
    env:
      NOTEPUB_VERSION: ` + version + `
      NOTEPUB_BIN: ./notepub
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Resolve content source config
        id: source
        env:
          SOURCE_RAW: ${{ vars.CONTENT_SOURCE }}
          CONTENT_REPO_RAW: ${{ vars.CONTENT_REPO }}
          CONTENT_REF_RAW: ${{ vars.CONTENT_REF }}
        run: |
          SOURCE="${SOURCE_RAW:-local}"
          case "$SOURCE" in
            local|content_repo|s3) ;;
            *)
              echo "Unsupported CONTENT_SOURCE: $SOURCE"
              echo "Allowed values: local, content_repo, s3"
              exit 1
              ;;
          esac

          CONTENT_REPO="${CONTENT_REPO_RAW:-}"
          CONTENT_REF="${CONTENT_REF_RAW:-main}"

          if [ "$SOURCE" = "content_repo" ] && [ -z "$CONTENT_REPO" ]; then
            echo "CONTENT_SOURCE=content_repo requires vars.CONTENT_REPO (owner/repo)"
            exit 1
          fi

          echo "content_source=$SOURCE" >> "$GITHUB_OUTPUT"
          echo "content_repo=$CONTENT_REPO" >> "$GITHUB_OUTPUT"
          echo "content_ref=$CONTENT_REF" >> "$GITHUB_OUTPUT"

      - name: Checkout content repository
        if: steps.source.outputs.content_source == 'content_repo'
        uses: actions/checkout@v4
        with:
          repository: ${{ steps.source.outputs.content_repo }}
          ref: ${{ steps.source.outputs.content_ref }}
          path: .external-content

      - name: Sync external content into site content/
        if: steps.source.outputs.content_source == 'content_repo'
        run: |
          find ./content -type f -name '*.md' ! -path './content/search.md' -delete

          rsync -a \
            --exclude '.git/' \
            --exclude '.github/' \
            --exclude '.obsidian/' \
            --exclude 'README.md' \
            --exclude 'LICENSE' \
            --exclude '.gitignore' \
            --exclude '.gitattributes' \
            ./.external-content/ ./content/

      - name: Prepare S3 config override
        if: steps.source.outputs.content_source == 's3'
        env:
          S3_ENDPOINT: ${{ vars.S3_ENDPOINT }}
          S3_REGION: ${{ vars.S3_REGION }}
          S3_BUCKET: ${{ vars.S3_BUCKET }}
          S3_PREFIX: ${{ vars.S3_PREFIX }}
          S3_USE_PATH_STYLE: ${{ vars.S3_USE_PATH_STYLE }}
          S3_ACCESS_KEY: ${{ secrets.S3_ACCESS_KEY }}
          S3_SECRET_KEY: ${{ secrets.S3_SECRET_KEY }}
        run: |
          if [ -z "${S3_ENDPOINT:-}" ] || [ -z "${S3_REGION:-}" ] || [ -z "${S3_BUCKET:-}" ]; then
            echo "S3 mode requires vars.S3_ENDPOINT, vars.S3_REGION, vars.S3_BUCKET"
            exit 1
          fi
          if [ -z "${S3_ACCESS_KEY:-}" ] || [ -z "${S3_SECRET_KEY:-}" ]; then
            echo "S3 mode requires secrets.S3_ACCESS_KEY and secrets.S3_SECRET_KEY"
            exit 1
          fi

          PREFIX="${S3_PREFIX:-content}"
          USE_PATH_STYLE="${S3_USE_PATH_STYLE:-true}"

          awk -v endpoint="$S3_ENDPOINT" \
              -v region="$S3_REGION" \
              -v bucket="$S3_BUCKET" \
              -v prefix="$PREFIX" \
              -v key="$S3_ACCESS_KEY" \
              -v secret="$S3_SECRET_KEY" \
              -v use_path_style="$USE_PATH_STYLE" '
            BEGIN { skip=0 }
            /^content:/ {
              print "content:"
              print "  source: \"s3\""
              print "  s3:"
              print "    endpoint: \"" endpoint "\""
              print "    region: \"" region "\""
              print "    bucket: \"" bucket "\""
              print "    prefix: \"" prefix "\""
              print "    access_key: \"" key "\""
              print "    secret_key: \"" secret "\""
              print "    use_path_style: " use_path_style
              skip=1
              next
            }
            skip && /^[A-Za-z_][A-Za-z0-9_]*:/ { skip=0 }
            !skip { print }
          ' config.yaml > config.effective.yaml

      - name: Set effective build config
        id: buildcfg
        run: |
          if [ "${{ steps.source.outputs.content_source }}" = "s3" ]; then
            echo "path=./config.effective.yaml" >> "$GITHUB_OUTPUT"
          else
            echo "path=./config.yaml" >> "$GITHUB_OUTPUT"
          fi

      - name: Download notepub binary
        run: |
          curl -L -o notepub "https://github.com/cookiespooky/notepub/releases/download/${NOTEPUB_VERSION}/notepub_linux_amd64"
          chmod +x notepub

      - name: Configure Pages
        id: pages
        uses: actions/configure-pages@v5

      - name: Build site
        env:
          NOTEPUB_CONFIG: ${{ steps.buildcfg.outputs.path }}
          NOTEPUB_BASE_URL: ${{ vars.NOTEPUB_BASE_URL }}
          NOTEPUB_MEDIA_BASE_URL: ${{ vars.NOTEPUB_MEDIA_BASE_URL }}
          GITHUB_PAGES_BASE_URL: ${{ steps.pages.outputs.base_url }}
        run: ./scripts/build.sh

      - name: Upload artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: ./dist

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
`
}

func legacyBuildScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BIN="${NOTEPUB_BIN:-notepub}"
CFG="${NOTEPUB_CONFIG:-./config.yaml}"
RULES="${NOTEPUB_RULES:-./rules.yaml}"
ART="./.notepub/artifacts"
OUT="./dist"
CONTENT_DIR="./content"
MEDIA_DIR="./media"

infer_custom_domain_base_url() {
  local cname="${ROOT}/CNAME"
  local domain=""

  if [[ ! -f "$cname" ]]; then
    return 1
  fi

  while IFS= read -r line; do
    line="${line%%#*}"
    line="${line//$'\r'/}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    if [[ -n "$line" ]]; then
      domain="$line"
      break
    fi
  done < "$cname"

  if [[ -z "$domain" ]]; then
    return 1
  fi

  domain="${domain#http://}"
  domain="${domain#https://}"
  domain="${domain%%/*}"
  if [[ -z "$domain" ]]; then
    return 1
  fi

  printf 'https://%s/' "$domain"
}

infer_github_pages_base_url() {
  local repo="${GITHUB_REPOSITORY:-}"
  local owner="${GITHUB_REPOSITORY_OWNER:-}"

  if [[ -z "$repo" ]]; then
    return 1
  fi
  if [[ -z "$owner" ]]; then
    owner="${repo%%/*}"
  fi

  local name="${repo#*/}"
  if [[ "$name" == "${owner}.github.io" ]]; then
    printf 'https://%s.github.io/' "$owner"
  else
    printf 'https://%s.github.io/%s/' "$owner" "$name"
  fi
}

if [[ -z "${NOTEPUB_BASE_URL:-}" && "${GITHUB_ACTIONS:-}" == "true" ]]; then
  if [[ -n "${GITHUB_PAGES_BASE_URL:-}" ]]; then
    export NOTEPUB_BASE_URL="${GITHUB_PAGES_BASE_URL%/}/"
    echo "Using GitHub Pages URL from configure-pages: $NOTEPUB_BASE_URL"
  elif BASE_URL="$(infer_custom_domain_base_url)"; then
    export NOTEPUB_BASE_URL="$BASE_URL"
    echo "Using custom domain URL from CNAME: $NOTEPUB_BASE_URL"
  elif BASE_URL="$(infer_github_pages_base_url)"; then
    export NOTEPUB_BASE_URL="$BASE_URL"
    echo "Using inferred GitHub Pages URL: $NOTEPUB_BASE_URL"
  fi
fi

if [[ -n "${NOTEPUB_BASE_URL:-}" && -z "${NOTEPUB_MEDIA_BASE_URL:-}" ]]; then
  export NOTEPUB_MEDIA_BASE_URL="${NOTEPUB_BASE_URL%/}/media/"
fi

if [[ -n "${NOTEPUB_BASE_URL:-}" ]]; then
  RESOLVED_CFG="./config.resolved.yaml"
  awk -v base_url="${NOTEPUB_BASE_URL%/}/" \
      -v media_base_url="${NOTEPUB_MEDIA_BASE_URL%/}/" '
    BEGIN {
      in_site = 0
      seen_site = 0
      seen_base = 0
      seen_media = 0
    }
    function finish_site() {
      if (in_site) {
        if (!seen_base) {
          print "  base_url: \"" base_url "\" # set by build.sh"
        }
        if (!seen_media) {
          print "  media_base_url: \"" media_base_url "\" # set by build.sh"
        }
      }
      in_site = 0
    }
    /^site:[[:space:]]*$/ {
      finish_site()
      in_site = 1
      seen_site = 1
      seen_base = 0
      seen_media = 0
      print
      next
    }
    in_site && /^[A-Za-z_][A-Za-z0-9_]*:/ {
      finish_site()
      print
      next
    }
    in_site && /^[[:space:]]*base_url:/ {
      print "  base_url: \"" base_url "\" # set by build.sh"
      seen_base = 1
      next
    }
    in_site && /^[[:space:]]*media_base_url:/ {
      print "  media_base_url: \"" media_base_url "\" # set by build.sh"
      seen_media = 1
      next
    }
    { print }
    END {
      finish_site()
      if (!seen_site) {
        print ""
        print "site:"
        print "  base_url: \"" base_url "\" # set by build.sh"
        print "  media_base_url: \"" media_base_url "\" # set by build.sh"
      }
    }
  ' "$CFG" > "$RESOLVED_CFG"
  CFG="$RESOLVED_CFG"
  echo "Using resolved build config: $CFG"
fi

if [[ -z "${NOTEPUB_BIN:-}" && -x "./notepub" ]]; then
  BIN="./notepub"
fi

if ! command -v "$BIN" >/dev/null 2>&1; then
  echo "notepub binary not found: $BIN"
  echo "Set NOTEPUB_BIN, for example:"
  echo "  NOTEPUB_BIN=/path/to/notepub $0"
  exit 1
fi

if [[ -d "$CONTENT_DIR" && -f "./scripts/normalize-obsidian-embeds.sh" ]]; then
  echo "[0/8] normalize obsidian embeds"
  chmod +x ./scripts/normalize-obsidian-embeds.sh
  ./scripts/normalize-obsidian-embeds.sh "$CONTENT_DIR"
fi

echo "[1/8] index"
"$BIN" index --config "$CFG" --rules "$RULES"

echo "[2/8] validate links + markdown"
VALIDATE_HELP="$("$BIN" validate --help 2>&1 || true)"
if printf '%s\n' "$VALIDATE_HELP" | grep -q -- "-links"; then
  "$BIN" validate --config "$CFG" --rules "$RULES" --links
else
  echo "validate --links is not supported by this notepub binary; skipping"
fi

if printf '%s\n' "$VALIDATE_HELP" | grep -q -- "-markdown"; then
  "$BIN" validate --config "$CFG" --rules "$RULES" --markdown --markdown-format text
else
  echo "validate --markdown is not supported by this notepub binary; skipping"
fi

echo "[3/8] build"
"$BIN" build --config "$CFG" --rules "$RULES" --artifacts "$ART" --dist "$OUT"

echo "[4/8] export content media"
rm -rf "$OUT/media"
mkdir -p "$OUT/media"

if [[ -d "$CONTENT_DIR" ]]; then
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --prune-empty-dirs \
      --exclude '.git/' \
      --exclude '.github/' \
      --exclude '.obsidian/' \
      --exclude '*.md' \
      "$CONTENT_DIR"/ "$OUT/media/"
  else
    find "$CONTENT_DIR" -type f ! -name '*.md' -print0 | while IFS= read -r -d '' f; do
      rel="${f#$CONTENT_DIR/}"
      mkdir -p "$OUT/media/$(dirname "$rel")"
      cp "$f" "$OUT/media/$rel"
    done
  fi
fi

if [[ -d "$MEDIA_DIR" ]]; then
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --prune-empty-dirs \
      --exclude '.git/' \
      --exclude '.github/' \
      --exclude '.obsidian/' \
      --exclude '*.md' \
      "$MEDIA_DIR"/ "$OUT/media/"
  else
    find "$MEDIA_DIR" -type f ! -name '*.md' -print0 | while IFS= read -r -d '' f; do
      rel="${f#$MEDIA_DIR/}"
      mkdir -p "$OUT/media/$(dirname "$rel")"
      cp "$f" "$OUT/media/$rel"
    done
  fi
fi

if [[ -f "./CNAME" ]]; then
  cp ./CNAME "$OUT/CNAME"
fi

echo "[5/8] copy llms files"
if [[ -f "$OUT/assets/llms.txt" ]]; then
  cp "$OUT/assets/llms.txt" "$OUT/llms.txt"
fi
if [[ -f "$OUT/assets/llms-full.txt" ]]; then
  cp "$OUT/assets/llms-full.txt" "$OUT/llms-full.txt"
fi

echo "[6/8] normalize robots"
if [[ -f "$OUT/robots.txt" ]]; then
  awk '!/^LLM: /' "$OUT/robots.txt" > "$OUT/robots.txt.tmp"
  cat "$OUT/robots.txt.tmp" > "$OUT/robots.txt"
  rm -f "$OUT/robots.txt.tmp"
fi

echo "[8/8] done -> $OUT"
`
}

func modernDeployWorkflow(name, version string) string {
	if strings.TrimSpace(name) == "" {
		name = "Deploy Notepub Site to GitHub Pages"
	}
	if strings.TrimSpace(version) == "" {
		version = "v0.1.3"
	}
	return `name: ` + name + `

on:
  push:
    branches: [main]
  workflow_dispatch:
  repository_dispatch:
    types: [content-updated]

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    runs-on: ubuntu-latest
    env:
      NOTEPUB_VERSION: ` + version + `
      NOTEPUB_BIN: ./.np/bin/notepub
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Resolve content source config
        id: source
        env:
          SOURCE_RAW: ${{ vars.CONTENT_SOURCE }}
          CONTENT_REPO_RAW: ${{ vars.CONTENT_REPO }}
          CONTENT_REF_RAW: ${{ vars.CONTENT_REF }}
        run: |
          SOURCE="${SOURCE_RAW:-local}"
          case "$SOURCE" in
            local|content_repo|s3) ;;
            *)
              echo "Unsupported CONTENT_SOURCE: $SOURCE"
              echo "Allowed values: local, content_repo, s3"
              exit 1
              ;;
          esac

          CONTENT_REPO="${CONTENT_REPO_RAW:-}"
          CONTENT_REF="${CONTENT_REF_RAW:-main}"

          if [ "$SOURCE" = "content_repo" ] && [ -z "$CONTENT_REPO" ]; then
            echo "CONTENT_SOURCE=content_repo requires vars.CONTENT_REPO (owner/repo)"
            exit 1
          fi

          echo "content_source=$SOURCE" >> "$GITHUB_OUTPUT"
          echo "content_repo=$CONTENT_REPO" >> "$GITHUB_OUTPUT"
          echo "content_ref=$CONTENT_REF" >> "$GITHUB_OUTPUT"

      - name: Resolve local content dir from config
        id: contentdir
        run: |
          CONTENT_DIR="$(python3 - <<'PY'
          import re
          from pathlib import Path

          cfg = Path("./.np/config.yaml")
          if not cfg.exists():
              print("./content")
              raise SystemExit(0)

          lines = cfg.read_text(encoding="utf-8").splitlines()
          in_content = False
          for line in lines:
              if re.match(r'^content:\s*$', line):
                  in_content = True
                  continue
              if in_content and re.match(r'^[A-Za-z_][A-Za-z0-9_]*:\s*$', line):
                  in_content = False
              if in_content:
                  m = re.match(r'^\s{2}local_dir:\s*(.+?)\s*$', line)
                  if m:
                      value = m.group(1).strip().strip('"').strip("'")
                      print(value or "./content")
                      raise SystemExit(0)

          print("./content")
          PY
          )"
          if [ -z "$CONTENT_DIR" ]; then
            CONTENT_DIR="./content"
          fi
          echo "path=$CONTENT_DIR" >> "$GITHUB_OUTPUT"

      - name: Checkout content repository
        if: steps.source.outputs.content_source == 'content_repo'
        uses: actions/checkout@v4
        with:
          repository: ${{ steps.source.outputs.content_repo }}
          ref: ${{ steps.source.outputs.content_ref }}
          path: .external-content

      - name: Sync external content into site content
        if: steps.source.outputs.content_source == 'content_repo'
        env:
          CONTENT_DIR: ${{ steps.contentdir.outputs.path }}
        run: |
          mkdir -p "$CONTENT_DIR"
          find "$CONTENT_DIR" -type f -name '*.md' ! -path "$CONTENT_DIR/search.md" -delete

          rsync -a \
            --exclude '.git/' \
            --exclude '.github/' \
            --exclude '.obsidian/' \
            --exclude 'README.md' \
            --exclude 'LICENSE' \
            --exclude '.gitignore' \
            --exclude '.gitattributes' \
            ./.external-content/ "$CONTENT_DIR/"

      - name: Prepare S3 config override
        if: steps.source.outputs.content_source == 's3'
        env:
          S3_ENDPOINT: ${{ vars.S3_ENDPOINT }}
          S3_REGION: ${{ vars.S3_REGION }}
          S3_BUCKET: ${{ vars.S3_BUCKET }}
          S3_PREFIX: ${{ vars.S3_PREFIX }}
          S3_USE_PATH_STYLE: ${{ vars.S3_USE_PATH_STYLE }}
          S3_ACCESS_KEY: ${{ secrets.S3_ACCESS_KEY }}
          S3_SECRET_KEY: ${{ secrets.S3_SECRET_KEY }}
        run: |
          if [ -z "${S3_ENDPOINT:-}" ] || [ -z "${S3_REGION:-}" ] || [ -z "${S3_BUCKET:-}" ]; then
            echo "S3 mode requires vars.S3_ENDPOINT, vars.S3_REGION, vars.S3_BUCKET"
            exit 1
          fi
          if [ -z "${S3_ACCESS_KEY:-}" ] || [ -z "${S3_SECRET_KEY:-}" ]; then
            echo "S3 mode requires secrets.S3_ACCESS_KEY and secrets.S3_SECRET_KEY"
            exit 1
          fi

          python3 - <<'PY'
          import os
          import re
          from pathlib import Path

          src = Path("./.np/config.yaml")
          dst = Path("./.np/config.effective.yaml")
          lines = src.read_text(encoding="utf-8").splitlines()

          out = []
          i = 0
          saw_content = False
          while i < len(lines):
              line = lines[i]
              if re.match(r"^s3:\s*$", line):
                  i += 1
                  while i < len(lines) and (lines[i].startswith(" ") or lines[i].strip() == ""):
                      i += 1
                  continue

              if re.match(r"^content:\s*$", line):
                  saw_content = True
                  out.append(line)
                  i += 1
                  saw_source = False
                  while i < len(lines):
                      nested = lines[i]
                      if re.match(r"^[A-Za-z_][A-Za-z0-9_]*:\s*$", nested):
                          break
                      if re.match(r"^\s{2}source:\s*", nested):
                          out.append('  source: "s3"')
                          saw_source = True
                      else:
                          out.append(nested)
                      i += 1
                  if not saw_source:
                      out.append('  source: "s3"')
                  continue

              out.append(line)
              i += 1

          if not saw_content:
              out.extend(["content:", '  source: "s3"'])

          out.extend(
              [
                  "",
                  "s3:",
                  f'  endpoint: "{os.environ["S3_ENDPOINT"]}"',
                  f'  region: "{os.environ["S3_REGION"]}"',
                  f'  force_path_style: {os.environ.get("S3_USE_PATH_STYLE", "true")}',
                  f'  bucket: "{os.environ["S3_BUCKET"]}"',
                  f'  prefix: "{os.environ.get("S3_PREFIX", "content")}"',
                  f'  access_key: "{os.environ["S3_ACCESS_KEY"]}"',
                  f'  secret_key: "{os.environ["S3_SECRET_KEY"]}"',
              ]
          )

          dst.write_text("\n".join(out) + "\n", encoding="utf-8")
          PY

      - name: Set effective build config
        id: buildcfg
        run: |
          if [ "${{ steps.source.outputs.content_source }}" = "s3" ]; then
            echo "path=./.np/config.effective.yaml" >> "$GITHUB_OUTPUT"
          else
            echo "path=./.np/config.yaml" >> "$GITHUB_OUTPUT"
          fi

      - name: Download notepub binary
        run: |
          mkdir -p ./.np/bin
          curl -fL -o ./.np/bin/notepub "https://github.com/cookiespooky/notepub/releases/download/${NOTEPUB_VERSION}/notepub_linux_amd64"
          chmod +x ./.np/bin/notepub
          ./.np/bin/notepub version

      - name: Configure Pages
        id: pages
        uses: actions/configure-pages@v5

      - name: Build site
        env:
          NOTEPUB_CONFIG: ${{ steps.buildcfg.outputs.path }}
          NOTEPUB_BASE_URL: ${{ vars.NOTEPUB_BASE_URL }}
          NOTEPUB_MEDIA_BASE_URL: ${{ vars.NOTEPUB_MEDIA_BASE_URL }}
          GITHUB_PAGES_BASE_URL: ${{ steps.pages.outputs.base_url }}
        run: ./.np/scripts/build.sh

      - name: Upload artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: ./.np/dist

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
`
}

func modernBuildScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

BIN="${NOTEPUB_BIN:-notepub}"
CFG="${NOTEPUB_CONFIG:-./.np/config.yaml}"
RULES="${NOTEPUB_RULES:-./.np/rules.yaml}"
ART="${NOTEPUB_ARTIFACTS_DIR:-./.notepub/artifacts}"
OUT="${NOTEPUB_DIST_DIR:-./.np/dist}"

resolve_content_dir() {
  python3 - "$CFG" <<'PY'
import re
import sys
from pathlib import Path

cfg = Path(sys.argv[1])
if not cfg.exists():
    print("./content")
    raise SystemExit(0)

lines = cfg.read_text(encoding="utf-8").splitlines()
in_content = False
for line in lines:
    if re.match(r'^content:\s*$', line):
        in_content = True
        continue
    if in_content and re.match(r'^[A-Za-z_][A-Za-z0-9_]*:\s*$', line):
        in_content = False
    if in_content:
        m = re.match(r'^\s{2}local_dir:\s*(.+?)\s*$', line)
        if m:
            value = m.group(1).strip().strip('"').strip("'")
            print(value or "./content")
            raise SystemExit(0)
print("./content")
PY
}

CONTENT_DIR="${NOTEPUB_CONTENT_DIR:-$(resolve_content_dir)}"
MEDIA_DIR="${NOTEPUB_MEDIA_DIR:-./media}"

infer_custom_domain_base_url() {
  local cname="${ROOT}/CNAME"
  local domain=""

  if [[ ! -f "$cname" ]]; then
    return 1
  fi

  while IFS= read -r line; do
    line="${line%%#*}"
    line="${line//$'\r'/}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    if [[ -n "$line" ]]; then
      domain="$line"
      break
    fi
  done < "$cname"

  if [[ -z "$domain" ]]; then
    return 1
  fi

  domain="${domain#http://}"
  domain="${domain#https://}"
  domain="${domain%%/*}"
  if [[ -z "$domain" ]]; then
    return 1
  fi

  printf 'https://%s/' "$domain"
}

infer_github_pages_base_url() {
  local repo="${GITHUB_REPOSITORY:-}"
  local owner="${GITHUB_REPOSITORY_OWNER:-}"

  if [[ -z "$repo" ]]; then
    return 1
  fi
  if [[ -z "$owner" ]]; then
    owner="${repo%%/*}"
  fi

  local name="${repo#*/}"
  if [[ "$name" == "${owner}.github.io" ]]; then
    printf 'https://%s.github.io/' "$owner"
  else
    printf 'https://%s.github.io/%s/' "$owner" "$name"
  fi
}

if [[ -z "${NOTEPUB_BASE_URL:-}" && "${GITHUB_ACTIONS:-}" == "true" ]]; then
  if [[ -n "${GITHUB_PAGES_BASE_URL:-}" ]]; then
    export NOTEPUB_BASE_URL="${GITHUB_PAGES_BASE_URL%/}/"
    echo "Using GitHub Pages URL from configure-pages: $NOTEPUB_BASE_URL"
  elif BASE_URL="$(infer_custom_domain_base_url)"; then
    export NOTEPUB_BASE_URL="$BASE_URL"
    echo "Using custom domain URL from CNAME: $NOTEPUB_BASE_URL"
  elif BASE_URL="$(infer_github_pages_base_url)"; then
    export NOTEPUB_BASE_URL="$BASE_URL"
    echo "Using inferred GitHub Pages URL: $NOTEPUB_BASE_URL"
  fi
fi

if [[ -n "${NOTEPUB_BASE_URL:-}" && -z "${NOTEPUB_MEDIA_BASE_URL:-}" ]]; then
  export NOTEPUB_MEDIA_BASE_URL="${NOTEPUB_BASE_URL%/}/media/"
fi

echo "[0/9] prepare settings"
if command -v python3 >/dev/null 2>&1; then
  CFG="$(python3 ./.np/scripts/prepare-settings.py notes "$CFG")"
else
  echo "python3 is required to prepare Notepub template settings"
  exit 1
fi

if [[ -n "${NOTEPUB_BASE_URL:-}" ]]; then
  RESOLVED_CFG="./.np/config.resolved.yaml"
  mkdir -p "$(dirname "$RESOLVED_CFG")"
  awk -v base_url="${NOTEPUB_BASE_URL%/}/" \
      -v media_base_url="${NOTEPUB_MEDIA_BASE_URL%/}/" '
    BEGIN {
      in_site = 0
      seen_site = 0
      seen_base = 0
      seen_media = 0
    }
    function finish_site() {
      if (in_site) {
        if (!seen_base) {
          print "  base_url: \"" base_url "\" # set by build.sh"
        }
        if (!seen_media) {
          print "  media_base_url: \"" media_base_url "\" # set by build.sh"
        }
      }
      in_site = 0
    }
    /^site:[[:space:]]*$/ {
      finish_site()
      in_site = 1
      seen_site = 1
      seen_base = 0
      seen_media = 0
      print
      next
    }
    in_site && /^[A-Za-z_][A-Za-z0-9_]*:/ {
      finish_site()
      print
      next
    }
    in_site && /^[[:space:]]*base_url:/ {
      print "  base_url: \"" base_url "\" # set by build.sh"
      seen_base = 1
      next
    }
    in_site && /^[[:space:]]*media_base_url:/ {
      print "  media_base_url: \"" media_base_url "\" # set by build.sh"
      seen_media = 1
      next
    }
    { print }
    END {
      finish_site()
      if (!seen_site) {
        print ""
        print "site:"
        print "  base_url: \"" base_url "\" # set by build.sh"
        print "  media_base_url: \"" media_base_url "\" # set by build.sh"
      }
    }
  ' "$CFG" > "$RESOLVED_CFG"
  CFG="$RESOLVED_CFG"
  echo "Using resolved build config: $CFG"
fi

if [[ -z "${NOTEPUB_BIN:-}" && -x "./.np/bin/notepub" ]]; then
  BIN="./.np/bin/notepub"
fi

if ! command -v "$BIN" >/dev/null 2>&1; then
  echo "notepub binary not found: $BIN"
  echo "Set NOTEPUB_BIN, for example:"
  echo "  NOTEPUB_BIN=/path/to/notepub $0"
  exit 1
fi

if [[ -f "./.np/scripts/generate-runtime-config.sh" ]]; then
  echo "[1/9] generate runtime config"
  ./.np/scripts/generate-runtime-config.sh "$CFG"
fi

echo "[2/9] index"
"$BIN" index --config "$CFG" --rules "$RULES"

echo "[3/9] validate links + markdown"
VALIDATE_HELP="$("$BIN" validate --help 2>&1 || true)"
if printf '%s\n' "$VALIDATE_HELP" | grep -q -- "-links"; then
  "$BIN" validate --config "$CFG" --rules "$RULES" --links
else
  echo "validate --links is not supported by this notepub binary; skipping"
fi

if printf '%s\n' "$VALIDATE_HELP" | grep -q -- "-markdown"; then
  "$BIN" validate --config "$CFG" --rules "$RULES" --markdown --markdown-format text
else
  echo "validate --markdown is not supported by this notepub binary; skipping"
fi

echo "[4/9] build"
"$BIN" build --config "$CFG" --rules "$RULES" --artifacts "$ART" --dist "$OUT"

echo "[5/9] export content media"
rm -rf "$OUT/media"
mkdir -p "$OUT/media"

if [[ -d "$CONTENT_DIR" ]]; then
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --prune-empty-dirs \
      --exclude '.git/' \
      --exclude '.github/' \
      --exclude '.obsidian/' \
      --exclude '*.md' \
      "$CONTENT_DIR"/ "$OUT/media/"
  else
    find "$CONTENT_DIR" -type f ! -name '*.md' -print0 | while IFS= read -r -d '' f; do
      rel="${f#$CONTENT_DIR/}"
      mkdir -p "$OUT/media/$(dirname "$rel")"
      cp "$f" "$OUT/media/$rel"
    done
  fi
fi

if [[ -d "$MEDIA_DIR" ]]; then
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --prune-empty-dirs \
      --exclude '.git/' \
      --exclude '.github/' \
      --exclude '.obsidian/' \
      --exclude '*.md' \
      "$MEDIA_DIR"/ "$OUT/media/"
  else
    find "$MEDIA_DIR" -type f ! -name '*.md' -print0 | while IFS= read -r -d '' f; do
      rel="${f#$MEDIA_DIR/}"
      mkdir -p "$OUT/media/$(dirname "$rel")"
      cp "$f" "$OUT/media/$rel"
    done
  fi
fi

if [[ -f "./CNAME" ]]; then
  cp ./CNAME "$OUT/CNAME"
fi

echo "[6/9] prepare icons manifest llms"
python3 ./.np/scripts/prepare-settings.py assets "$CFG" "$OUT"

echo "[7/9] normalize robots"
if [[ -f "$OUT/robots.txt" ]]; then
  awk '!/^LLM: /' "$OUT/robots.txt" > "$OUT/robots.txt.tmp"
  cat "$OUT/robots.txt.tmp" > "$OUT/robots.txt"
  rm -f "$OUT/robots.txt.tmp"
fi

echo "[8/9] mirror dist to ./dist (compat)"
rm -rf ./dist
mkdir -p ./dist
cp -R "$OUT"/. ./dist/

echo "[9/9] done -> $OUT"
`
}
