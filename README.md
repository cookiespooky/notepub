# Notepub

Single-binary publishing engine for Markdown content from local folders or S3.

## Repository purpose

This repository is the engine core and includes:

- CLI (`cmd/notepub`)
- indexing, validation, serving and static build pipelines
- embedded fallback theme
- runnable sandbox in `examples/dev-sandbox`

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
notepub help
notepub version
```

Advanced usage:

```bash
notepub index --config /path/to/config.yaml --rules /path/to/rules.yaml
notepub serve --config /path/to/config.yaml --rules /path/to/rules.yaml
notepub build --config /path/to/config.yaml --rules /path/to/rules.yaml --dist ./dist
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --links
```

## Release binaries

GitHub Release publishes Linux binaries from `.github/workflows/release.yml`:

- `notepub_linux_amd64`
- `notepub_linux_arm64`

Usage example:

```bash
NOTEPUB_VERSION=v0.1.0
curl -L -o notepub https://github.com/cookiespooky/notepub/releases/download/${NOTEPUB_VERSION}/notepub_linux_amd64
chmod +x notepub
./notepub validate --config ./config.yaml --rules ./rules.yaml
./notepub build --config ./config.yaml --rules ./rules.yaml --dist ./dist
```

## Config and rules

- `config.example.yaml` is a runnable reference and points to `examples/dev-sandbox`.
- `rules.example.yaml` is a generic reference.
- runtime artifacts are stored under `paths.file_root` (default `/var/lib/notepub`).

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
