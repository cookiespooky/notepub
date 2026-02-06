---
type: guide
slug: configuration
title: Configuration
description: Configure Notepub (site, content, S3, cache, theme, and rules).
hub: docs
draft: false
---

# Configuration

Notepub reads `config.yaml` and `rules.yaml` to control indexing, routing, and rendering.

## Config sources (priority)

1) CLI flags (`--config`, `--rules`)
2) env paths (`CONFIG_PATH`, `RULES_PATH`)
3) files in repo root (`config.yaml`, `rules.yaml`)

## Required fields

- `site.base_url` (must be an absolute URL)
- `content.source`: `"s3"` or `"local"`
- `s3.bucket` when using S3

## Common settings

- `site.media_base_url`: optional. When set, `/media/*` is rewritten to this base in **serve** and **build**.
- `theme.*`: theme directory and subfolders.
- `paths.*`: artifacts, snapshot, cache locations.
- `cache.*`: HTML cache TTL and stale settings.

## Local content

```
content:
  source: "local"
  local_dir: "./markdown"
```

## S3 content

```
content:
  source: "s3"

s3:
  endpoint: "https://s3.amazonaws.com" # or custom endpoint
  region: "us-east-1"
  bucket: "your-bucket"
  prefix: ""
  access_key: "..."
  secret_key: "..."
  anonymous: false
```

## Rules

`rules.yaml` defines:

- **Types**: `type -> template + permalink`
- **Links**: how relations are resolved (`resolve_by: "wikimap"` for Obsidian compatibility)
- **Collections**: filter/forward/backrefs lists
- **Validation**: strictness for duplicates, unknown types, etc.

If you keep the default `rules.yaml`, make sure every content note has:

```
---
type: page|guide|hub|home
slug: your-slug
title: Your Title
---
```

Next: [Commands](/commands/).
