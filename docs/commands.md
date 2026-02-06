---
type: guide
slug: commands
title: Commands
description: CLI usage for index, serve, build, validate, and troubleshooting.
hub: docs
draft: false
---

# Commands

## Index

```
notepub index
notepub index --config /path/to/config.yaml --rules /path/to/rules.yaml
```

Generates `resolve.json`, `search.json`, sitemaps, robots, and collections.

## Serve

```
notepub serve
notepub serve --config /path/to/config.yaml --rules /path/to/rules.yaml
```

Starts HTTP server with live markdown rendering.

## Build

```
notepub build --dist ./dist
notepub build --config /path/to/config.yaml --rules /path/to/rules.yaml --dist ./dist
```

Builds a static site into `dist/`.

## Validate

```
notepub validate
notepub validate --links
```

Validates config + rules, and optionally checks links via `resolve.json`.

## Help / Version

```
notepub help
notepub help build
notepub version
```

## Exit codes

- `0` success
- `1` runtime/config/content errors
- `2` usage errors
