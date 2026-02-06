---
type: guide
slug: build-and-deploy
title: Build & Deploy
description: Build static output and deploy to GitHub Pages or a server.
hub: docs
draft: false
---

# Build & Deploy

## Build a static site

```
notepub build --dist ./dist
```

Notes:
- Uses the same render pipeline as `serve`.
- If `artifacts/resolve.json` is missing, `build` runs `index` automatically (unless `--no-index`).
- Copies theme assets into `dist/assets`.
- Copies `sitemap*.xml`, `robots.txt`, and `search.json` when present.

## GitHub Pages

1) Ensure `site.base_url` matches your Pages URL (e.g. `https://user.github.io/repo/`).
2) Commit `dist/` and push to `main`.
3) Set Pages → Source = GitHub Actions.

## Server deploy

Run `notepub serve` behind a reverse proxy (Nginx/Caddy) and enable HTTPS.

## Systemd

Use `deploy/systemd/` sample units:

- `notepub.service`
- `notepub-index.service`
- `notepub-index.timer`

## Nginx

See `deploy/nginx/notepub.conf` for a sample configuration.
