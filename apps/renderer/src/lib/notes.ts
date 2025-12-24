import matter from "gray-matter";
import path from "path";
import { markdownToHtml } from "./markdown";
import { fetchObject, listNoteObjects } from "./s3";
import { getS3Config } from "./config";
import { slugifySegment } from "./slug";
import { CategoryIndex, FlatNoteIndex, IndexData, NoteResponse, S3ObjectEntry } from "./types";

export async function getIndexData(prefix?: string, opts?: { includeDrafts?: boolean }): Promise<IndexData> {
  const includeDrafts = opts?.includeDrafts ?? false;
  const objects = await listNoteObjects(prefix);
  const sortedObjects = sortEntries(objects);
  const rawNotes = await loadRawNotes(sortedObjects, prefix);
  const slugLookup = buildSlugLookup(rawNotes);

  const flat: FlatNoteIndex[] = [];
  for (const note of rawNotes) {
    if (note.isDraft && !includeDrafts) continue;
    const html = await markdownToHtml(note.content, {
      objectKey: note.entry.key,
      s3Prefix: prefix,
      slugLookup,
    });
    const breadcrumbs = buildBreadcrumbs(note.category, note.slug, note.title);
    flat.push({
      key: note.entry.key,
      relativeKey: note.relativeKey,
      title: note.title,
      slug: note.slug,
      category: note.category,
      categorySlug: note.category ? slugifySegment(note.category) || null : null,
      tags: note.tags,
      html,
      preview: note.preview,
      created: note.created,
      updated: note.updated,
      breadcrumbs,
      etag: note.entry.etag,
      isHome: note.isHome,
      isDraft: note.isDraft,
    });
  }

  const categories = buildCategories(flat);
  return { categories, flat };
}

export type RawNote = {
  entry: S3ObjectEntry;
  relativeKey: string;
  title: string;
  baseName: string;
  slug: string;
  category: string | null;
  tags: string[];
  created: string | null;
  updated: string | null;
  content: string;
  preview: string;
  aliases: string[];
  isHome: boolean;
  isDraft: boolean;
  frontmatter: Record<string, unknown>;
};

export async function getRawNotes(prefix?: string, opts?: { includeDrafts?: boolean }): Promise<RawNote[]> {
  const includeDrafts = opts?.includeDrafts ?? false;
  const objects = await listNoteObjects(prefix);
  const sortedObjects = sortEntries(objects);
  const rawNotes = await loadRawNotes(sortedObjects, prefix);
  return includeDrafts ? rawNotes : rawNotes.filter((note) => !note.isDraft);
}

export async function getNoteBySlug(
  slug: string,
  prefix?: string,
  opts?: { includeDrafts?: boolean },
): Promise<NoteResponse | null> {
  const index = await getIndexData(prefix, opts);
  const entry = index.flat.find((item) => item.slug === slug);
  if (!entry) return null;
  return {
    slug: entry.slug,
    title: entry.title,
    category: entry.category,
    html: entry.html,
    preview: entry.preview,
    tags: entry.tags,
    created: entry.created,
    updated: entry.updated,
    breadcrumbs: entry.breadcrumbs,
    isDraft: entry.isDraft,
  };
}

type SlugLookup = {
  byPath: Map<string, string>;
  byName: Map<string, string[]>;
  byAlias: Map<string, string[]>;
  folderIndexByName: Map<string, string[]>;
};

async function loadRawNotes(objects: S3ObjectEntry[], prefix?: string): Promise<RawNote[]> {
  const notes: RawNote[] = [];
  for (const entry of objects) {
    const remote = await fetchObject(entry.key);
    const markdown = remote.body;
    const parsed = matter(markdown);
    const frontmatter = (parsed.data ?? {}) as Record<string, unknown>;
    const fmSlug = typeof frontmatter.slug === "string" ? frontmatter.slug.trim() : "";
    const relativeKeyRaw = stripPrefix(entry.key, prefix);
    const relativeKey = safeDecode(relativeKeyRaw);
    const baseName = path.posix.basename(relativeKey, path.extname(relativeKey));
    const slug = fmSlug && !fmSlug.includes("/") ? fmSlug : baseName;
    const title = deriveTitle(relativeKey, frontmatter);
    const category = deriveCategoryFromPath(relativeKey);
    const tags = normalizeTags(frontmatter.tags);
    const created = frontmatter.created ? String(frontmatter.created) : null;
    const updated = frontmatter.updated ? String(frontmatter.updated) : remote.lastModified || entry.lastModified || null;
    const aliases = normalizeAliases(frontmatter.aliases);
    const isRoot = relativeKey.split("/").filter(Boolean).length === 1;
    const isHome = isRoot && frontmatter.home === true;
    const isDraft = parseDraftFlag(frontmatter.draft);
    notes.push({
      entry,
      relativeKey,
      title,
      baseName,
      slug,
      category,
      tags,
      created,
      updated,
      content: parsed.content,
      preview: buildPreview(parsed.content),
      aliases,
      isHome,
      isDraft,
      frontmatter,
    });
  }
  return notes;
}

export function buildSlugLookup(notes: RawNote[]): SlugLookup {
  const byPath = new Map<string, string>();
  const byName = new Map<string, string[]>();
  const byAlias = new Map<string, string[]>();
  const folderIndexByName = new Map<string, string[]>();

  for (const note of notes) {
    const normalizedPath = note.relativeKey.endsWith(".md") ? note.relativeKey : `${note.relativeKey}.md`;
    byPath.set(normalizedPath, note.slug);
    const nameKey = note.baseName.trim().toLowerCase();
    const existing = byName.get(nameKey) || [];
    existing.push(normalizedPath);
    byName.set(nameKey, existing);

    const titleKey = note.title.trim().toLowerCase();
    if (titleKey) {
      const arr = byAlias.get(titleKey) || [];
      arr.push(normalizedPath);
      byAlias.set(titleKey, arr);
    }
    for (const alias of note.aliases) {
      const key = alias.trim().toLowerCase();
      if (!key) continue;
      const arr = byAlias.get(key) || [];
      arr.push(normalizedPath);
      byAlias.set(key, arr);
    }

    const slugAlias = note.slug.trim().toLowerCase();
    if (slugAlias) {
      const arr = byAlias.get(slugAlias) || [];
      arr.push(normalizedPath);
      byAlias.set(slugAlias, arr);
    }
  }
  return { byPath, byName, byAlias, folderIndexByName };
}

function deriveTitle(key: string, frontmatter: Record<string, unknown>) {
  const fmTitle = typeof frontmatter.title === "string" ? frontmatter.title.trim() : "";
  if (fmTitle) return fmTitle;
  const decoded = safeDecode(key);
  const base = path.posix.basename(decoded, path.extname(decoded));
  return base;
}

function deriveCategoryFromPath(relativeKey: string): string | null {
  const parts = relativeKey.split("/").filter(Boolean);
  if (parts.length <= 1) return null;
  // Everything except the last segment (filename) is treated as category path.
  return parts.slice(0, -1).join("/");
}

function normalizeTags(input: unknown): string[] {
  if (!input) return [];
  if (Array.isArray(input)) return input.map(String);
  if (typeof input === "string") {
    return input
      .split(",")
      .map((tag) => tag.trim())
      .filter(Boolean);
  }
  return [];
}

function normalizeAliases(input: unknown): string[] {
  if (!input) return [];
  if (Array.isArray(input)) return input.map(String).filter(Boolean);
  if (typeof input === "string") return input.split(",").map((s) => s.trim()).filter(Boolean);
  return [];
}

function parseDraftFlag(input: unknown): boolean {
  if (input === true) return true;
  if (typeof input === "string") {
    const normalized = input.trim().toLowerCase();
    return normalized === "true" || normalized === "yes" || normalized === "1";
  }
  if (typeof input === "number") {
    return input === 1;
  }
  return false;
}

function buildPreview(content: string, limit = 200) {
  const withoutCode = content.replace(/```[\s\S]*?```/g, " ");
  const withoutLinks = withoutCode.replace(/\[([^\]]+)\]\([^)]+\)/g, "$1");
  const plain = withoutLinks
    .replace(/[*_`>#-]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (plain.length <= limit) return plain;
  return `${plain.slice(0, limit).trim()}…`;
}

function sortEntries(objects: S3ObjectEntry[]) {
  return [...objects].sort((a, b) => a.key.localeCompare(b.key));
}

function buildCategories(flat: FlatNoteIndex[]): CategoryIndex[] {
  const groups = new Map<string, CategoryIndex>();
  for (const note of flat) {
    const name = note.category?.trim();
    if (!name) continue;
    const key = name.toLowerCase();
    if (!groups.has(key)) {
      const slug = slugifySegment(name) || "category";
      groups.set(key, { name, slug, notes: [] });
    }
    groups.get(key)!.notes.push({
      title: note.title,
      slug: note.slug,
      isDraft: note.isDraft,
      isHome: note.isHome,
    });
  }
  const sorted = [...groups.values()].sort((a, b) => {
    return a.name.localeCompare(b.name, "ru");
  });
  sorted.forEach((cat) =>
    cat.notes.sort((a, b) => {
      if (a.isHome && !b.isHome) return -1;
      if (b.isHome && !a.isHome) return 1;
      return a.title.localeCompare(b.title, "ru");
    }),
  );
  return sorted;
}

function buildBreadcrumbs(category: string | null, slug: string, title: string) {
  const breadcrumbs: { title: string; href: string | null }[] = [];
  if (category) {
    const categorySlug = slugifySegment(category);
    breadcrumbs.push({ title: category, href: categorySlug ? `/category/${categorySlug}` : null });
  }
  breadcrumbs.push({ title, href: `/${slug}` });
  return breadcrumbs;
}

function stripPrefix(key: string, customPrefix?: string) {
  const { prefix } = getS3Config();
  const effectivePrefix = customPrefix || prefix;
  if (!effectivePrefix) return key.replace(/^\/+/, "");
  const normalizedPrefix = effectivePrefix.replace(/^\/+/, "").replace(/\/+$/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedKey.startsWith(normalizedPrefix + "/")) {
    return normalizedKey.slice(normalizedPrefix.length + 1);
  }
  return normalizedKey;
}

function safeDecode(value: string) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}
