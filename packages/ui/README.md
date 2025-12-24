# @notepub/ui

Shared UI hooks and helpers for renderer/editor parity.

## Hooks

- `useAutosaveNote` — manages dirty state, debounced autosave (default 3s), and manual save for a note `{ path, frontmatter, body }`. Respects the current `draft` flag; autosave never triggers publish side-effects. Blocks when publish is in flight; validates before saving.
- `useNoteLinks` — resolves note links (slug/path/Obsidian-style `[[...]]`) to hrefs and provides a click handler to intercept navigation inside the editor while keeping renderer behavior.

These hooks are intended to be shared between the renderer and the editor so preview, navigation, and saving semantics stay consistent with the live site.
