---
type: guide
slug: themes
title: Themes
description: Theme structure, templates, assets, and fallback behavior.
hub: docs
draft: false
---

# Themes

Themes control your site layout and styles.

## Directory structure

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

## How templates are selected

- `layout.html` wraps all pages.
- `page.html` is the default body template.
- If a template matches the content type (e.g. `guide.html`), it is used.
- `home.html` is used when the route is `/`.
- `search.html` and `category.html` are used when present.

## Assets

Static assets in your theme are served from `/assets/*`.
If files are missing, the embedded fallback theme is used.

## Template data

Useful fields:
- `.Title`, `.Canonical`, `.BaseURL`, `.AssetsBase`
- `.Page.Type`, `.Page.Slug`, `.Page.Title`
- `.Collections` (runtime collections)
- `.Meta.OpenGraph` and `.Meta.JSONLD`
