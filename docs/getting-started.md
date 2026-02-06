---
type: guide
slug: getting-started
title: Getting Started
description: Install Notepub, configure it, and publish your first site.
hub: docs
draft: false
---

# Getting Started

This guide gets you from zero to a working Notepub site.

## 1) Install

Build from source:

```
# from repo root
go build -o notepub ./cmd/notepub
```

Or use a prebuilt binary if you have one.

## 2) Create config

Copy the example and edit:

```
cp config.example.yaml config.yaml
cp rules.example.yaml rules.yaml
```

Minimum required:

- `site.base_url`
- `content.source` (`s3` or `local`)
- `s3.bucket` if using S3

## 3) Prepare content

Local content:

- Put markdown files in `./markdown` (default).
- Use frontmatter with `type`, `slug`, and `title`.

S3 content:

- Upload your markdown into the bucket and prefix.
- Set `s3.endpoint`, `s3.bucket`, credentials (or `s3.anonymous: true`).

## 4) Index

```
./notepub index
```

This creates `artifacts/resolve.json` and search/sitemap artifacts.

## 5) Serve

```
./notepub serve
```

Open `http://127.0.0.1:8080` (or `server.listen` in config).

## 6) Build static site (optional)

```
./notepub build --dist ./dist
```

Now `dist/` contains a static site for GitHub Pages or any static host.

Next: [Configuration](/configuration/) and [Commands](/commands/).
