import matter from "gray-matter";
import { slugifySegment } from "./slug";
import { deleteObject, listFolderPlaceholders, putObject } from "./s3";
import { getRawNotes, RawNote } from "./notes";

export type FrontmatterInput = Record<string, unknown> & {
  title?: string;
  slug?: string;
  category?: string | null;
  home?: boolean;
  draft?: boolean;
};

export type SaveParams = {
  frontmatter: FrontmatterInput;
  body: string;
  path?: string;
  prefix?: string;
  /** Override destination path; defaults to buildNotePath(frontmatter). */
  targetPath?: string;
};

export type SaveResult = {
  path: string;
  slug: string;
};

export function buildNotePath(frontmatter: FrontmatterInput) {
  const rawSlug = (frontmatter.slug || "").toString().trim();
  const slug = rawSlug || "";
  const category = (frontmatter.category || "").toString().trim();
  const categorySegment = category ? `${slugifySegment(category) || category}/` : "";
  return `${categorySegment}${slug}.md`;
}

export function serializeNote(frontmatter: FrontmatterInput, body: string) {
  const fm = { ...frontmatter };
  if (typeof fm.draft === "undefined") {
    fm.draft = true;
  }
  return matter.stringify(body, fm).trim() + "\n";
}

export async function saveNote({ frontmatter, body, path, prefix, targetPath: overridePath }: SaveParams): Promise<SaveResult> {
  const targetPath = overridePath || path || buildNotePath(frontmatter);
  const content = serializeNote(frontmatter, body);
  await putObject(targetPath, content, prefix);
  if (path && path !== targetPath) {
    await deleteObject(path, prefix).catch(() => {
      // Ignore delete errors to avoid blocking save; caller can clean up later.
    });
  }
  return { path: targetPath, slug: String(frontmatter.slug || "") };
}

export function findNoteBySlug(notes: RawNote[], slug: string) {
  return notes.find((note) => note.slug === slug);
}

export function slugExists(notes: RawNote[], slug: string, excludePath?: string) {
  const normalizedSlug = slug.trim();
  if (!normalizedSlug) return false;
  const normalize = (p: string) => (p.endsWith(".md") ? p : `${p}.md`);
  return notes.some((note) => {
    if (note.slug !== normalizedSlug) return false;
    if (!excludePath) return true;
    // Skip the current file (relativeKey or full key match).
    const normalizedExclude = normalize(excludePath);
    if (note.relativeKey === excludePath || note.entry.key === excludePath) return false;
    if (note.relativeKey === normalizedExclude || note.entry.key === normalizedExclude) return false;
    return true;
  });
}

export async function listRawNotesWithDrafts(prefix?: string) {
  return getRawNotes(prefix, { includeDrafts: true });
}

export async function listFolders(prefix?: string) {
  const placeholders = await listFolderPlaceholders(prefix);
  const folders = new Set<string>();
  for (const entry of placeholders) {
    const key = entry.key.replace(/\/\.keep$/i, "");
    const cleaned = key.replace(/^\/+|\/+$/g, "");
    if (cleaned && !cleaned.startsWith(".np-assets") && !cleaned.startsWith(".")) {
      folders.add(cleaned);
    }
  }
  return [...folders];
}
