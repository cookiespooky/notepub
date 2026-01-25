# Notepub (self-hosted MVP)

Single-binary self-hosted publishing engine for Obsidian notes stored in your S3 bucket.
[https://cookiespooky.github.io/notepub](https://cookiespooky.github.io/notepub)

## How it works

- `notepub index` lists S3, diffs `snapshot/objects.json`, and generates:
  - `artifacts/resolve.json`
  - `artifacts/search.json`
  - `artifacts/sitemap-index.xml` + `artifacts/sitemap-0001.xml`...
  - `artifacts/robots.txt`
  - `artifacts/collections/*.json` (if enabled + materialized)
- `notepub serve` reads `resolve.json` from disk (in-memory cache, reload on mtime), renders markdown from S3, and serves HTML.
- `notepub build` renders markdown from S3 to static HTML in `dist/` (GitHub Pages-friendly).
- Theme assets are served from `/assets/*` (theme directory). Fallback embedded theme if not found.
- Wiki-links (`[[...]]`) are rewritten to standard links at render time using `resolve.json` (title + filename lookup).
- Obsidian image embeds `![[...]]` are rewritten to `/media/*` and served via S3 presign redirect.
- `/media/*` only serves media keys referenced by indexed markdown by default; set `media.expose_all_under_prefix: true` to allow all media under the prefix.

## Paths

Defaults:
- `paths.file_root`: `/var/lib/notepub`
- `paths.artifacts_dir`: `/var/lib/notepub/artifacts`
- `paths.snapshot_file`: `/var/lib/notepub/snapshot/objects.json`
- `paths.cache_root`: `/var/cache/notepub`

Artifacts:
```
/var/lib/notepub/
  artifacts/
    resolve.json
    search.json
    sitemap-index.xml
    sitemap-0001.xml ...
    robots.txt
    collections/
      <name>.json
  snapshot/
    objects.json
```

## Config

Copy `config.example.yaml` to `./config.yaml` in the repo root and edit.

Config sources (priority):
1) CLI flags (`--config`, `--rules`)
2) env paths (`CONFIG_PATH`, `RULES_PATH`)
3) files in repo root (`config.yaml`, `rules.yaml`)

Required fields:
- `site.base_url`
- `site.media_base_url` (optional, used by build for absolute media URLs)
- `s3.bucket` (credentials depend on mode)

Listen address comes from `server.listen` in config (default `:8081`).

Notes:
- `s3.prefix` is normalized to no leading slash and optional trailing slash.
- `s3.region` and `s3.force_path_style` are supported.
- If `s3.access_key`/`s3.secret_key` are omitted, the AWS default credential chain is used.
- For public buckets, set `s3.anonymous: true` to disable signing.
- `content.source` switches markdown source: `"s3"` or `"local"`. If `s3.bucket` is empty, local is used.
- `content.local_dir` defaults to `./markdown` and is resolved relative to `config.yaml`.
- `rules_path` points to `rules.yaml` (defaults to `rules.yaml` next to `config.yaml`).
- `site.media_base_url` if unset keeps existing absolute media URLs and falls back to `/media/*` links.
- `site.base_url` may include a non-root path (e.g., `https://user.github.io/repo/`).
- Environment variables do not override values inside `config.yaml`.

## GitHub Pages deploy

This repo is set up to deploy the static site from `dist/` on every push to `main`.

Steps:
1) Ensure `site.base_url` matches your Pages URL, e.g. `https://cookiespooky.github.io/notepub/`.
2) Commit the built site in `dist/` (the workflow uploads it as-is).
3) In GitHub, set Pages → Source = GitHub Actions.

The workflow is in `.github/workflows/deploy.yml` and expects the site to already exist in `dist/`.

## Rules (universal engine)

`rules.yaml` defines how notes are normalized and routed.

Key ideas:
- core fields: `type`, `slug`, `title`, `description`
- mapping/derivation order comes from `fields` + `derive`
- `types` maps content type → template + permalink

Minimal example:
```
version: 1
fields:
  type: "type"
  slug: "slug"
  title: "title"
  description: "description"
derive:
  slug: ["slug", "filename"]
  title: ["title", "h1", "filename"]
  description: ["description", "excerpt"]
types:
  page:
    template: "page.html"
    permalink: "/{{ slug }}/"
  category:
    template: "category.html"
    permalink: "/category/{{ slug }}/"
  home:
    template: "home.html"
    permalink: "/"
defaults:
  type: "page"
  template: "page.html"
  permalink: "/{{ slug }}/"
```

Template data:
- `.Page.Type` and `.Page.Slug` are available in all templates.
- `.Template` selects the body template when a matching file exists.

## Commands

Index (CLI):
```
notepub index
notepub index --config /path/to/config.yaml --rules /path/to/rules.yaml
```

Serve (HTTP):
```
notepub serve
notepub serve --config /path/to/config.yaml --rules /path/to/rules.yaml
```

Build (static):
```
notepub build --dist ./dist
notepub build --config /path/to/config.yaml --rules /path/to/rules.yaml --dist ./dist
```

Validate (config + rules, optional resolve.json, link checks):
```
notepub validate
notepub validate --links
notepub validate --config /path/to/config.yaml --rules /path/to/rules.yaml --links
```

Help:
```
notepub help
notepub help build
```

Version:
```
notepub version
```

Metrics & health:
- `/health` returns `ok`
- `/metrics` returns JSON counters (expvar)

Exit codes:
- `0` success
- `1` runtime/config/content errors (S3, parse, index/build, rules validation, etc.)
- `2` usage errors (unknown flags, missing command/args)

Error precedence:
1) if `--rules` is provided, the file is validated before config load
2) if config is missing, error is `config file not found: <path>`
3) otherwise, rules are resolved via config/adjacent/cwd and validated

Build notes:
- Uses the same render pipeline as `serve`.
- Defaults to `artifacts/` and `dist/` next to `rules.yaml` (override with `--artifacts` / `--dist`).
- If `artifacts/resolve.json` is missing, `build` runs `index` automatically (unless `--no-index`).
- Copies theme assets to `dist/assets`.
- Copies `sitemap*.xml`, `robots.txt`, and `search.json` from `artifacts/` when present.
- Generates minimal `sitemap.xml` and `robots.txt` if they are missing.
- Redirects (301) are written as `dist/<from>/index.html` with meta refresh + canonical.
- Relative media links are resolved via `site.media_base_url` when set.
- Generates `dist/404.html` using the theme's `notfound.html` (or a plain "Not Found").

## Search Mode Contract

Notepub search is dual-mode by design and must keep `/v1/search` available.

Principles:
- Dual-mode search: themes should support server search (`/v1/search`) and static search (`/search.json`).
- Static hosting (build): default to `search.json` because `/v1/search` does not exist.
- Server mode (serve): default to `/v1/search`; static index may be absent.
- Do not remove `/v1/search` or make search JS-only.
- `/search` must render without JS (SSR results), JS adds autocomplete/enhanced results.

Static index format (`/search.json`):
```
{
  "generated_at": "RFC3339",
  "items": [
    { "title": "...", "path": "/...", "snippet": "...", "type": "page", "updatedAt": "RFC3339" }
  ]
}
```

## Local dev (macOS, no Docker)

Local MinIO setup that matches `config.local.yaml`:

1) Install MinIO (Homebrew):
```
brew install minio
```

2) Run MinIO with a data directory:
```
minio server /path/to/minio-data --address ":9000" --console-address ":9001"
```

3) Create the bucket expected by `config.local.yaml` (default: `local-bucket`):
```
brew install minio-mc
mc alias set local http://localhost:9000 minioadmin minioadmin
mc mb local/local-bucket
```

4) Run Notepub locally:
```
./notepub index --config ./config.local.yaml
./notepub serve --config ./config.local.yaml
```

If your bucket name, credentials, or MinIO endpoint differ, update `config.local.yaml` accordingly.

## CI / GitHub Actions

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

## Systemd

See `deploy/systemd/` for `notepub.service`, `notepub-index.service`, `notepub-index.timer`.

Example `Environment=` entries (only if config is not in the repo root):
```
Environment=CONFIG_PATH=/etc/notepub/config.yaml
Environment=RULES_PATH=/etc/notepub/rules.yaml
```

## Nginx

Sample config in `deploy/nginx/notepub.conf`.

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
