---
type: guide
slug: overview
title: Overview
description: High-level view of Notepub, its data flow, and core concepts.
hub: docs
draft: false
---

# Overview

Notepub is a single-binary publishing engine for Obsidian notes. It reads markdown from S3 or a local folder,
builds an index, then renders HTML using a theme. You can run it as a server or build a static site.

## How it works

**Index**
- `notepub index` scans content, diffs `snapshot/objects.json`, and generates:
  - `artifacts/resolve.json`
  - `artifacts/search.json`
  - `artifacts/sitemap-index.xml` + `artifacts/sitemap-0001.xml`...
  - `artifacts/robots.txt`
  - `artifacts/collections/*.json` (if enabled + materialized)

**Serve**
- `notepub serve` reads `resolve.json` from disk (in-memory cache, reload on mtime), renders markdown, and serves HTML.
- Theme assets are served from `/assets/*`.

**Build**
- `notepub build` renders markdown to static HTML in `dist/` (GitHub Pages-friendly).

## Key ideas

- **Rules-driven routing**: `rules.yaml` defines types, permalinks, validation, and collections.
- **Wikilinks**: `[[...]]` become standard links using an Obsidian-like wikimap
  (filename basename → aliases → title, then fallback to slug/path).
- **Media**: `![[...]]` images map to `/media/*`. If `site.media_base_url` is set, URLs are rewritten to the CDN.
- **Search**: server search (`/v1/search`) and static search (`/search.json`) are both supported.

## What you need to know

- **Required config**: `site.base_url` is mandatory. Choose `content.source: s3|local`.
- **Local vs S3**: If `s3.bucket` is empty, Notepub defaults to local content.
- **Deploy choices**: Serve for dynamic rendering, or build for static hosting.

Next: [Getting Started](/getting-started/).
