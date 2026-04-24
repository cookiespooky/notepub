# Changelog

All notable changes to this project are documented in this file.

## Unreleased

### Added

- Introduced `internal/mdproc` with shared markdown preprocessing utilities:
  - line ending normalization
  - safe rewrite outside fenced/inline code
  - code masking helpers for extraction stages
- Added parser option `WithAutoHeadingID()` in markdown renderer for stable heading anchors.
- Added markdown renderer support for:
  - Obsidian-style callouts (`> [!type]`, folded `+/-`)
  - Obsidian inline syntax (`==highlight==`, `~sub~`, `^sup^`)
  - footnotes
  - math wrappers for inline/block formulas
- Added support for raw HTML passthrough in markdown rendering.
- Added configurable markdown HTML policy in config:
  - `markdown.html_policy: safe` (default sanitizer)
  - `markdown.html_policy: unsafe`
  - `markdown.html_policy: deny` (diagnostics-level enforcement)
- Added `rules.links.kind: tags` extraction for Obsidian-style body tags.
- Added frontmatter link parsing for markdown link syntax in `value_syntax` (`markdown_link`, `markdown_image`, and `auto` detection).
- Added markdown diagnostics engine with typed codes and line-level reporting.
- Added new validate flags:
  - `--markdown`
  - `--markdown-strict`
  - `--markdown-format text|json`
  - `--output <path>` (write diagnostics report to file)
- Added `notepub template check` and `notepub template update` for updating template build infrastructure with dry-run/apply modes.
- Added regression tests for markdown preprocessing and extraction behavior:
  - `internal/mdproc/mdproc_test.go`
  - `internal/indexer/markdown_extract_test.go`
  - `internal/serve/markdown_test.go`
  - `internal/indexer/diagnostics_test.go`

### Changed

- Unified markdown preprocessing behavior between runtime `serve` and static `build`.
- Hardened wikilink/image preprocessing to avoid modifying content inside code fences and inline code spans.
- Updated image-embed handling so `![[...]]` is treated as an image only when target has an image extension.
- Updated non-image embed handling so `![[...]]` renders as linked embed block output.
- Updated OG image extraction to skip non-image embeds and prefer actual image targets.
- Updated media allowlist extraction to ignore non-image embeds and markdown/image syntax inside code blocks.
- Updated README Obsidian section to reflect default Obsidian-first behavior.
- Updated README with markdown diagnostics usage and strict validation examples.
- Updated README with JSON diagnostics output example for CI integration.
- Updated README with HTML policy behavior and file output example.

### Fixed

- Fixed false-positive Obsidian embed parsing that could leak into SEO metadata (for example incorrect `og:image`).
- Fixed link/media extraction false positives from fenced code and inline code snippets.

### Release

- Added Windows arm64 release binary and inject tag version into release builds.
