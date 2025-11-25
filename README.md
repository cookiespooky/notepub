## Obsidian + Timeweb S3

Next.js (App Router) site that renders Obsidian notes directly from a Timeweb S3 bucket and keeps them fresh via S3 ETags. No static rebuilds — everything is SSR + cache.

### Setup

1) Copy `.env.example` to `.env.local` and fill in your credentials/bucket:

```
S3_ENDPOINT=https://s3.timeweb.com
S3_REGION=ru-1
S3_BUCKET=your-bucket
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_PREFIX=website_notes/   # only files under this prefix are read
```

2) Install deps and run:

```
npm install
npm run dev
```

### How it works

- `/api/index` lists Markdown files via `listObjectsV2`, hashes the listing (key+etag+mtime) and caches the computed navigation + flat index on disk (`/cache/index.json`). If the listing hash is unchanged, cached data is returned.
- `/api/notes/[slug]` resolves the slug to the corresponding S3 key, checks cached note data by ETag, and only re-downloads the file when the ETag changes. Markdown is converted to HTML with remark-gfm + rehype-highlight; Obsidian callouts are styled.
- Assets linked relatively inside notes are turned into signed S3 URLs so images render without making the bucket public.
- The UI fetches `/api/index` for client-side search and renders notes server-side at `/[...slug]`.
