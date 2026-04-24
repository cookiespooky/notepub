# Notepub

Single-binary publishing engine for Markdown content from local folders or S3.

## Repository purpose

This repository is the engine core and includes:

- CLI (`cmd/notepub`)
- indexing, validation, serving and static build pipelines
- embedded fallback theme
- runnable sandbox in `examples/dev-sandbox`

## Documentation

For up-to-date user documentation, use:

- RU: https://cookiespooky.github.io/np/ru/docs/
- EN: https://cookiespooky.github.io/np/en/docs/

## Quick smoke run

```bash
go build -o notepub ./cmd/notepub
./notepub validate --config ./examples/dev-sandbox/config.yaml --rules ./examples/dev-sandbox/rules.yaml
./notepub index --config ./examples/dev-sandbox/config.yaml --rules ./examples/dev-sandbox/rules.yaml
./notepub serve --config ./examples/dev-sandbox/config.yaml --rules ./examples/dev-sandbox/rules.yaml
```

Open `http://127.0.0.1:8080`.

## Build static output

```bash
./notepub build --config ./examples/dev-sandbox/config.yaml --rules ./examples/dev-sandbox/rules.yaml --dist ./dist
```

## Commands

```bash
notepub index
notepub serve
notepub build
notepub validate
notepub template check
notepub template update --apply
notepub help
notepub version
```

Advanced usage:

```bash
notepub index --config /path/to/config.yaml --rules /path/to/rules.yaml
notepub serve --config /path/to/config.yaml --rules /path/to/rules.yaml
notepub build --config /path/to/config.yaml --rules /path/to/rules.yaml --dist ./dist
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --links
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --resolve ./artifacts/resolve.json --markdown
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --resolve ./artifacts/resolve.json --markdown --markdown-strict
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --resolve ./artifacts/resolve.json --markdown --markdown-format json
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --resolve ./artifacts/resolve.json --markdown --markdown-format json --output ./artifacts/markdown-diagnostics.json
```

## Template updates

Existing template repositories can update their build infrastructure for newer Notepub releases:

```bash
notepub template check
notepub template update
notepub template update --apply
```

`template update` is a dry run by default. `--apply` updates recognized infrastructure files such as GitHub Pages workflow, build script, runtime URL config, and generated config ignores. It leaves content and custom theme files unchanged, and reports manual review items such as root-absolute asset links or duplicate markdown titles.

## Obsidian support

`notepub` now renders Obsidian syntax by default (no feature toggles required).

What is supported in-engine:

- safer wikilink/embed preprocessing outside fenced and inline code
- `[[...]]` wikilinks and `![[...]]` embeds in markdown body
- `![[...]]` image embeds are converted only for real image targets
- non-image embeds are rendered as linked embed blocks
- Obsidian inline syntax: `==highlight==`, `~sub~`, `^sup^`
- Obsidian callouts (`> [!note]`, foldable `+/-` variants)
- footnotes and math wrappers (`$...$`, `$$...$$`)
- raw HTML in markdown body is preserved
- consistent preprocessing path for `serve` and `build`
- automatic heading IDs for markdown headings (anchor-friendly links)

Indexing and link extraction:

- `rules.links.kind: wikilinks` continues to work for body wikilinks.
- `rules.links.kind: tags` is supported to extract Obsidian-style body tags (`#tag`, `#parent/child`).
- frontmatter link field parsing supports `value_syntax: auto|wikilink|markdown_link|markdown_image`.

Important notes:

- CLI still does not mutate source markdown files.
- project-level normalize scripts are optional and only needed for legacy pipelines.

HTML policy:

- `markdown.html_policy: safe` (default) — raw HTML is sanitized.
- `markdown.html_policy: unsafe` — raw HTML is rendered as-is.
- `markdown.html_policy: deny` — raw HTML is treated as invalid in diagnostics.

## Markdown diagnostics

`validate` supports markdown diagnostics with explicit codes and line numbers:

- `NP-MD-WIKI-MISSING`
- `NP-MD-EMBED-MISSING`
- `NP-MD-WIKI-AMBIGUOUS`
- `NP-MD-HTML-SANITIZED`
- `NP-MD-RAW-HTML-UNSAFE`
- `NP-MD-RAW-HTML-DENY`
- `NP-MD-HTML-DANGEROUS`
- `NP-MD-READ-ERROR`
- `NP-MD-FRONTMATTER-ERROR`

Run diagnostics:

```bash
notepub validate --resolve ./artifacts/resolve.json --markdown
```

Strict mode (warnings fail the command):

```bash
notepub validate --resolve ./artifacts/resolve.json --markdown --markdown-strict
```

JSON output for CI:

```bash
notepub validate --resolve ./artifacts/resolve.json --markdown --markdown-format json
```

Write diagnostics to file:

```bash
notepub validate --resolve ./artifacts/resolve.json --markdown --markdown-format json --output ./artifacts/markdown-diagnostics.json
```

## Release binaries

GitHub Release publishes cross-platform binaries from `.github/workflows/release.yml`:

- `notepub_linux_amd64`
- `notepub_linux_arm64`
- `notepub_darwin_amd64`
- `notepub_darwin_arm64`
- `notepub_windows_amd64.exe`
- `notepub_windows_arm64.exe`
- `checksums.txt`

Usage example (Linux amd64):

```bash
NOTEPUB_VERSION=v0.1.1
curl -L -o notepub https://github.com/cookiespooky/notepub/releases/download/${NOTEPUB_VERSION}/notepub_linux_amd64
chmod +x notepub
./notepub validate --config ./config.yaml --rules ./rules.yaml
./notepub build --config ./config.yaml --rules ./rules.yaml --dist ./dist
```

macOS Apple Silicon:

```bash
NOTEPUB_VERSION=v0.1.1
curl -L -o notepub https://github.com/cookiespooky/notepub/releases/download/${NOTEPUB_VERSION}/notepub_darwin_arm64
chmod +x notepub
./notepub version
```

Windows PowerShell:

```powershell
$env:NOTEPUB_VERSION="v0.1.1"
Invoke-WebRequest -Uri "https://github.com/cookiespooky/notepub/releases/download/$env:NOTEPUB_VERSION/notepub_windows_amd64.exe" -OutFile ".\notepub.exe"
.\notepub.exe version
```

## Config and rules

- `config.example.yaml` is a runnable reference and points to `examples/dev-sandbox`.
- `rules.example.yaml` is a generic reference.
- runtime artifacts are stored under `paths.file_root` (default `/var/lib/notepub`).
- URL mode switching is handled by `runtime.mode: dev|prod` with `runtime.dev` / `runtime.prod` URL overrides.

Runtime URL resolution:

- `runtime.mode: prod` (default) uses `runtime.prod.base_url` / `runtime.prod.media_base_url` when set, otherwise falls back to `site.base_url` / `site.media_base_url`.
- `runtime.mode: dev` uses `runtime.dev.*` values first, then infers base URL from `server.listen`, and finally falls back to `site.*`.
- canonical and OpenGraph URLs are generated from the resolved base/media URLs, so `index`, `serve`, and `build` use the same normalization path.

## Example project

`examples/dev-sandbox` contains:

- `content/` Markdown and media
- `theme/` templates and assets
- `rules.yaml` and `config.yaml`
- sample Pages workflow for static deploy

## Health and metrics

- `/health` returns `ok`
- `/metrics` exposes expvar counters

## CI / GitHub Actions (advanced)

Pass config paths via env and keep secrets in the config file (checked in or generated in CI):
```
CONFIG_PATH=./config.yaml
RULES_PATH=./rules.yaml
```

To keep secrets out of git, write `config.yaml` from CI secrets at build time.
Example (GitHub Actions step):
```
cat > config.yaml <<'EOF'
site:
  base_url: "https://example.com"
s3:
  bucket: "${S3_BUCKET}"
  access_key: "${S3_ACCESS_KEY}"
  secret_key: "${S3_SECRET_KEY}"
EOF
```

## Theme

Theme directory structure:
```
<themes_dir>/<theme_name>/
  templates/
    layout.html
    page.html
    home.html
    notfound.html
  assets/
    styles.css
    home.css
    favicon.ico
    ...
```

If files are missing, the embedded fallback theme is used.

## Collections

Collections are defined in `rules.yaml` and can be materialized to JSON for fast reads.

If `artifacts.collections.enabled: true` and a collection has `materialize: true`,
the indexer writes `artifacts/collections/<name>.json` with `items` or `groups`.

Use collections in templates via `data.Collections` (runtime) or precomputed JSON
files for static consumption.


## Contact

Telegram: [cookiespooky](https://t.me/cookiespooky)
