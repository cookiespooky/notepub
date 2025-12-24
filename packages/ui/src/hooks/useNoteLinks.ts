import type React from "react";

export type NoteRef = {
  slug: string;
  title?: string;
  category?: string;
  path?: string;
};

export type ResolvedLink = {
  href: string;
  slug?: string;
  label?: string;
  external?: boolean;
};

type UseNoteLinksOptions = {
  notes: NoteRef[];
  onNavigate?: (slug: string) => void;
  basePath?: string;
};

const WIKILINK_RE = /\[\[([^[\]]+)\]\]/g;

/**
 * Resolves internal note links (slug, /path, or Obsidian-style wikilinks) to renderer/editor hrefs.
 * Editor can use handleClick to intercept navigation and load notes without a full page reload.
 */
export function useNoteLinks({ notes, onNavigate, basePath = "/" }: UseNoteLinksOptions) {
  const normalizeSlug = (value: string) => value.replace(/^\/+/, "").replace(/\/+$/, "");

  const findBySlugOrTitle = (value: string) => {
    const slugValue = normalizeSlug(value);
    return (
      notes.find((n) => normalizeSlug(n.slug) === slugValue) ||
      notes.find((n) => n.title && n.title.toLowerCase() === value.toLowerCase())
    );
  };

  const resolveWikiTarget = (raw: string) => {
    const [target, alias] = raw.split("|").map((part) => part.trim());
    return { target, alias: alias || undefined };
  };

  const toHref = (slug: string) => {
    const normalized = normalizeSlug(slug);
    if (!normalized) return basePath;
    return normalized.startsWith("/") ? normalized : `${basePath.replace(/\/$/, "")}/${normalized}`;
  };

  const resolveHref = (input: string): ResolvedLink => {
    const trimmed = input.trim();
    if (!trimmed) return { href: basePath };

    // Full URL
    if (/^https?:\/\//i.test(trimmed) || trimmed.startsWith("//")) {
      return { href: trimmed, external: true };
    }

    // Obsidian-style [[...]] (first occurrence)
    const wikiMatch = trimmed.match(/^\[\[(.+)\]\]$/);
    if (wikiMatch) {
      const { target, alias } = resolveWikiTarget(wikiMatch[1]);
      const found = findBySlugOrTitle(target);
      if (found) {
        return { href: toHref(found.slug), slug: found.slug, label: alias || found.title || found.slug };
      }
      return { href: toHref(target), slug: normalizeSlug(target), label: alias || target };
    }

    // Absolute path or slug
    if (trimmed.startsWith("/")) {
      const normalized = normalizeSlug(trimmed);
      const found = findBySlugOrTitle(normalized);
      return { href: toHref(trimmed), slug: found?.slug };
    }

    const found = findBySlugOrTitle(trimmed);
    if (found) {
      return { href: toHref(found.slug), slug: found.slug };
    }

    // Fallback to passthrough
    return { href: trimmed };
  };

  const handleClick = (event: React.MouseEvent<HTMLElement>, link: ResolvedLink) => {
    if (link.external || !link.slug) return;
    event.preventDefault();
    onNavigate?.(link.slug);
  };

  /**
    * Utility: replace wikilinks in raw markdown-ish text with resolved hrefs/labels.
    * Useful if you need to pre-process before rendering.
    */
  const replaceWikiLinks = (text: string) =>
    text.replace(WIKILINK_RE, (_match, inner) => {
      const { target, alias } = resolveWikiTarget(inner);
      const resolved = resolveHref(`[[${inner}]]`);
      const label = alias || resolved.label || target;
      return `[${label}](${resolved.href})`;
    });

  return {
    resolveHref,
    handleClick,
    replaceWikiLinks,
  };
}
