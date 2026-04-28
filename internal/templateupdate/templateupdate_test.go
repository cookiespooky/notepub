package templateupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyUpdateDryRunDoesNotWrite(t *testing.T) {
	root := writeLegacyProject(t)
	before := readFile(t, filepath.Join(root, "config.yaml"))

	report, err := Update(UpdateOptions{Root: root})
	if err != nil {
		t.Fatalf("Update dry-run: %v", err)
	}
	if !strings.Contains(report, "Mode: dry-run") {
		t.Fatalf("expected dry-run report, got %q", report)
	}
	after := readFile(t, filepath.Join(root, "config.yaml"))
	if after != before {
		t.Fatalf("dry-run changed config")
	}
}

func TestLegacyUpdateApplyWritesInfrastructure(t *testing.T) {
	root := writeLegacyProject(t)

	report, err := Update(UpdateOptions{Root: root, Apply: true})
	if err != nil {
		t.Fatalf("Update apply: %v", err)
	}
	if !strings.Contains(report, "Updated template infrastructure.") {
		t.Fatalf("expected apply report, got %q", report)
	}
	cfg := readFile(t, filepath.Join(root, "config.yaml"))
	if !strings.Contains(cfg, "media_base_url:") || !strings.Contains(cfg, "runtime:") || !strings.Contains(cfg, "overrides:") || !strings.Contains(cfg, "settings:") {
		t.Fatalf("config was not updated:\n%s", cfg)
	}
	if !strings.Contains(readFile(t, filepath.Join(root, "Site.md")), "site_title: \"Site\"") {
		t.Fatalf("Site.md was not created from config")
	}
	if !strings.Contains(readFile(t, filepath.Join(root, "Interface.md")), "ui_search_placeholder: Search documentation") {
		t.Fatalf("Interface.md was not created")
	}
	build := readFile(t, filepath.Join(root, "scripts", "build.sh"))
	if !strings.Contains(build, "config.resolved.yaml") || !strings.Contains(build, "GITHUB_PAGES_BASE_URL") {
		t.Fatalf("build script was not updated")
	}
	workflow := readFile(t, filepath.Join(root, ".github", "workflows", "deploy.yml"))
	if !strings.Contains(workflow, "name: Custom Deploy") {
		t.Fatalf("workflow name was not preserved:\n%s", workflow)
	}
	if !strings.Contains(workflow, "GITHUB_PAGES_BASE_URL: ${{ steps.pages.outputs.base_url }}") {
		t.Fatalf("workflow does not pass GitHub Pages URL:\n%s", workflow)
	}
	ignore := readFile(t, filepath.Join(root, ".gitignore"))
	if !strings.Contains(ignore, "config.resolved.yaml") {
		t.Fatalf("gitignore was not updated")
	}
}

func TestCheckReportsManualFindings(t *testing.T) {
	root := writeLegacyProject(t)
	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !strings.Contains(report, "duplicate markdown title may cause wikimap collision") {
		t.Fatalf("expected duplicate title finding:\n%s", report)
	}
	if !strings.Contains(report, "root-absolute /assets/logo.svg") {
		t.Fatalf("expected root absolute finding:\n%s", report)
	}
	if !strings.Contains(report, "settings fallback is missing") {
		t.Fatalf("expected settings fallback status:\n%s", report)
	}
}

func TestModernUpdateApplyWritesInfrastructure(t *testing.T) {
	root := writeModernProject(t)

	report, err := Update(UpdateOptions{Root: root, Apply: true})
	if err != nil {
		t.Fatalf("Update apply: %v", err)
	}
	if !strings.Contains(report, "Updated template infrastructure.") {
		t.Fatalf("expected apply report, got %q", report)
	}
	cfg := readFile(t, filepath.Join(root, ".np", "config.yaml"))
	if !strings.Contains(cfg, "media_base_url:") || !strings.Contains(cfg, "runtime:") || !strings.Contains(cfg, "overrides:") || !strings.Contains(cfg, "settings:") {
		t.Fatalf("config was not updated:\n%s", cfg)
	}
	build := readFile(t, filepath.Join(root, ".np", "scripts", "build.sh"))
	if !strings.Contains(build, "CONTENT_DIR=\"${NOTEPUB_CONTENT_DIR:-$(resolve_content_dir)}\"") {
		t.Fatalf("build script was not updated to config-driven content dir")
	}
	workflow := readFile(t, filepath.Join(root, ".github", "workflows", "deploy.yml"))
	if !strings.Contains(workflow, "Resolve local content dir from config") {
		t.Fatalf("modern workflow was not updated:\n%s", workflow)
	}
	if !strings.Contains(workflow, "target = (cfg.parent / target).resolve()") {
		t.Fatalf("modern workflow content dir resolution should be config-relative:\n%s", workflow)
	}
	if !strings.Contains(workflow, "path: ./.np/dist") {
		t.Fatalf("modern workflow artifact path missing:\n%s", workflow)
	}
	if !strings.Contains(build, "target = (cfg.parent / target).resolve()") {
		t.Fatalf("modern build script content dir resolution should be config-relative")
	}
}

func writeLegacyProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "scripts"))
	mkdir(t, filepath.Join(root, ".github", "workflows"))
	mkdir(t, filepath.Join(root, "theme", "templates"))
	mkdir(t, filepath.Join(root, "content"))
	writeFile(t, filepath.Join(root, "config.yaml"), `site:
  id: default
  base_url: "http://127.0.0.1:8080/"
  title: "Site"
content:
  source: "local"
  local_dir: "./content"
theme:
  dir: "."
  name: "theme"
rules_path: "./rules.yaml"
`)
	writeFile(t, filepath.Join(root, "rules.yaml"), `version: 1
`)
	writeFile(t, filepath.Join(root, "scripts", "build.sh"), `#!/usr/bin/env bash
notepub index --config ./config.yaml
notepub build --config ./config.yaml
`)
	writeFile(t, filepath.Join(root, ".github", "workflows", "deploy.yml"), `name: Custom Deploy
jobs:
  build:
    env:
      NOTEPUB_VERSION: v0.1.9
`)
	writeFile(t, filepath.Join(root, ".gitignore"), "dist/\n")
	writeFile(t, filepath.Join(root, "theme", "templates", "layout.html"), `<img src="/assets/logo.svg">`)
	writeFile(t, filepath.Join(root, "content", "home.md"), "---\ntitle: Same\n---\n")
	writeFile(t, filepath.Join(root, "content", "about.md"), "---\ntitle: Same\n---\n")
	return root
}

func writeModernProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, ".np", "scripts"))
	mkdir(t, filepath.Join(root, ".github", "workflows"))
	mkdir(t, filepath.Join(root, "content"))
	writeFile(t, filepath.Join(root, ".np", "config.yaml"), `site:
  id: modern
  base_url: "http://127.0.0.1:8080/"
  title: "Modern Site"
content:
  source: "local"
  local_dir: "../content"
theme:
  dir: "./.np"
  name: "theme"
rules_path: "./.np/rules.yaml"
`)
	writeFile(t, filepath.Join(root, ".np", "rules.yaml"), `version: 1
`)
	writeFile(t, filepath.Join(root, ".np", "scripts", "build.sh"), `#!/usr/bin/env bash
echo old
`)
	writeFile(t, filepath.Join(root, ".github", "workflows", "deploy.yml"), `name: Custom Modern Deploy
jobs:
  build:
    env:
      NOTEPUB_VERSION: v0.1.5
`)
	writeFile(t, filepath.Join(root, ".gitignore"), "dist/\n")
	return root
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
