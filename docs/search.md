---
type: guide
slug: search
title: Search
description: Search modes, endpoint behavior, and static index format.
hub: docs
draft: false
---

# Search

Notepub search is dual-mode by design and keeps `/v1/search` available.

## Modes

- **Server mode (serve)**: `/v1/search` returns JSON results.
- **Static mode (build)**: `/search.json` is used by the frontend.

## Principles

- Themes should support both modes.
- `/search` should work without JS (SSR results), JS adds autocomplete.
- Do not remove `/v1/search`.

## Static index format

```
{
  "generated_at": "RFC3339",
  "items": [
    { "title": "...", "path": "/...", "snippet": "...", "type": "page", "updatedAt": "RFC3339", "score": 1.25 }
  ]
}
```

## Server endpoint

`GET /v1/search?q=term&limit=10`

Returns:

```
{
  "items": [...],
  "next_cursor": "..."
}
```
