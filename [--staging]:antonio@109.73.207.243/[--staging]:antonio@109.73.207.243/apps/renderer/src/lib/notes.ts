import matter from "gray-matter";
import path from "path";
import { markdownToHtml } from "./markdown";
import { fetchObject, listNoteObjects } from "./s3";
import { getS3Config } from "./config";
import { resolveFolderSlugs } from "./slug";
import { FlatNoteIndex, FolderListing, FolderMeta, IndexData, IndexTreeNode, NoteResponse, S3ObjectEntry } from "./types";

const FOLDER_RENDER_VERSION = "9-folder-raw-slug";

export async function getIndexData(prefix?: string): Promise<IndexData> {
  const objects = await listNoteObjects(prefix);
  const folderMeta = await buildFolderMeta(objects, prefix);
  const sortedObjects = sortForFolderMeta(objects);
  const rawNotes = await loadRawNotes(sortedObjects, folderMeta, prefix);
  const slugLookup = buildSlugLookup(rawNotes);
  const flat: FlatNoteIndex[] = [];
  for (const note of rawNotes) {
    const html = await markdownToHtml(note.content, {
      objectKey: note.entry.key,
      folderMeta,
      s3Prefix: prefix,
      slugLookup,
    });
    const breadcrumbs = buildBreadcrumbs(note.relativeKey, folderMeta, note.slug, note.title, prefix);
    flat.push({
      key: note.entry.key,
      relativeKey: note.relativeKey,
      title: note.title,
      slug: note.slug,
      tags: note.tags,
      html,
      preview: note.preview,
      created: note.created,
      updated: note.updated,
      breadcrumbs,
      etag: note.entry.etag,
      isFolderIndex: note.isFolderIndex,
      isHome: note.isHome,
    });
  }

  const tree = buildTree(flat, folderMeta);
  return { tree, flat, folderMeta };
}

export async function getNoteBySlug(slug: string, prefix?: string): Promise<NoteResponse | null> {
  const index = await getIndexData(prefix);
  const entry = index.flat.find((item) => item.slug === slug);
  if (!entry) return null;
  return {
    slug: entry.slug,
    title: entry.title,
    html: entry.html,
    tags: entry.tags,
    created: entry.created,
    updated: entry.updated,
    breadcrumbs: entry.breadcrumbs,
  };
}

export async function getFolderBySlugPath(slugPath: string[], prefix?: string): Promise<FolderListing | null> {
  const index = await getIndexData(prefix);
  const target = findFolderNodeBySlugPath(index.tree, index.folderMeta, slugPath);
  if (!target) return null;

  const folderSlugs = resolveFolderSlugs(target.path, index.folderMeta);
  if (!folderSlugs) return null;
  const accumulatedCrumbs: { title: string; href: string | null }[] = [];
  for (let i = 0; i < target.path.length; i++) {
    const href = `/folders/${folderSlugs.slice(0, i + 1).join("/")}`;
    const meta = index.folderMeta.get(target.path.slice(0, i + 1).join("/"));
    accumulatedCrumbs.push({ title: meta?.title || target.path[i], href });
  }

  return {
    title: target.title,
    path: target.path,
    slugPath,
    breadcrumbs: accumulatedCrumbs,
    folders: (target.folders || [])
      .map((f) => {
        const slugs = resolveFolderSlugs(f.path, index.folderMeta);
        if (!slugs) return null;
        return { title: f.title, slugPath: slugs };
      })
      .filter(Boolean) as { title: string; slugPath: string[] }[],
    notes: target.children.map((n) => ({ title: n.title, slug: n.slug })),
  };
}

type RawNote = {
  entry: S3ObjectEntry;
  relativeKey: string;
  title: string;
  baseName: string;
  folderSegments: string[];
  slug: string;
  tags: string[];
  created: string | null;
  updated: string | null;
  content: string;
  preview: string;
  aliases: string[];
  isFolderIndex: boolean;
  isHome: boolean;
};

type SlugLookup = {
  byPath: Map<string, string>;
  byName: Map<string, string[]>;
  byAlias: Map<string, string[]>;
  folderIndexByName: Map<string, string[]>;
};

async function loadRawNotes(objects: S3ObjectEntry[], folderMeta: Map<string, FolderMeta>, prefix?: string): Promise<RawNote[]> {
  const notes: RawNote[] = [];
  for (const entry of objects) {
    const remote = await fetchObject(entry.key);
    const markdown = remote.body;
    const parsed = matter(markdown);
    const frontmatter = (parsed.data ?? {}) as Record<string, unknown>;
    const fmSlug = typeof frontmatter.slug === "string" ? frontmatter.slug.trim() : "";
    if (!fmSlug) {
      // Skip notes without an explicit slug in frontmatter to avoid implicit slug generation.
      continue;
    }
    const relativeKeyRaw = stripPrefix(entry.key, prefix);
    const relativeKey = safeDecode(relativeKeyRaw);
    const baseName = path.posix.basename(relativeKey, path.extname(relativeKey));
    const folderSegments = relativeKey.split("/").filter(Boolean).slice(0, -1);
    const isFolderIdx = baseName.toLowerCase() === "index";
    const isRoot = folderSegments.length === 0;

    if (isFolderIdx && folderSegments.length > 0) {
      const folderPath = folderSegments.join("/");
      const existing = folderMeta.get(folderPath) || { path: folderPath };
      const metaSlug = typeof frontmatter.slug === "string" && frontmatter.slug.trim().length > 0 ? frontmatter.slug.trim() : existing.slug;
      const metaTitle = existing.title || folderSegments.at(-1) || "";
      folderMeta.set(folderPath, { ...existing, slug: metaSlug, title: metaTitle, path: folderPath });
    }

    const folderSlugs = resolveFolderSlugs(folderSegments, folderMeta);
    if (!folderSlugs) {
      // Skip notes in folders without explicit slugs.
      continue;
    }

    const slug = buildSlugFromFolders(folderSlugs, fmSlug);
    const title = deriveTitle(relativeKey);
    const tags = normalizeTags(frontmatter.tags);
    const created = frontmatter.created ? String(frontmatter.created) : null;
    const updated = frontmatter.updated ? String(frontmatter.updated) : remote.lastModified || entry.lastModified || null;
    const aliases = normalizeAliases(frontmatter.aliases);
    const isHome = isRoot && frontmatter.home === true;
    notes.push({
      entry,
      relativeKey,
      title,
      baseName,
      folderSegments,
      slug,
      tags,
      created,
      updated,
      content: parsed.content,
      preview: buildPreview(parsed.content),
      aliases,
      isFolderIndex: isFolderIdx,
      isHome,
    });
  }
  return notes;
}

function buildSlugLookup(notes: RawNote[]): SlugLookup {
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

    // Treat folder index as alias of folder name
    if (note.baseName.toLowerCase() === "index" && note.folderSegments.length > 0) {
      const folderName = note.folderSegments[note.folderSegments.length - 1];
      if (folderName) {
        const folderKey = folderName.trim().toLowerCase();
        const arr = byAlias.get(folderKey) || [];
        arr.push(normalizedPath);
        byAlias.set(folderKey, arr);
        const folderIndexArr = folderIndexByName.get(folderKey) || [];
        folderIndexArr.push(normalizedPath);
        folderIndexByName.set(folderKey, folderIndexArr);
      }
    }
    if (note.baseName.toLowerCase() === "index" && note.folderSegments.length === 0) {
      // root index is alias for "index"
      const arr = byAlias.get("index") || [];
      arr.push(normalizedPath);
      byAlias.set("index", arr);
    }

    // Allow frontmatter slug text to be used as alias
    const slugAlias = note.slug.trim().toLowerCase();
    if (slugAlias) {
      const arr = byAlias.get(slugAlias) || [];
      arr.push(normalizedPath);
      byAlias.set(slugAlias, arr);
    }
  }
  return { byPath, byName, byAlias, folderIndexByName };
}

function deriveTitle(key: string) {
  const decoded = safeDecode(key);
  const base = path.posix.basename(decoded, path.extname(decoded));
  return base;
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

function sortForFolderMeta(objects: S3ObjectEntry[]) {
  return [...objects].sort((a, b) => {
    const aIsFolderIndex = a.key.toLowerCase().endsWith("/index.md");
    const bIsFolderIndex = b.key.toLowerCase().endsWith("/index.md");
    if (aIsFolderIndex && !bIsFolderIndex) return -1;
    if (!aIsFolderIndex && bIsFolderIndex) return 1;
    return a.key.localeCompare(b.key);
  });
}

function buildTree(flat: FlatNoteIndex[], folderMeta: Map<string, FolderMeta>): IndexTreeNode[] {
  const root: IndexTreeNode = { title: "", path: [], children: [], folders: [] };
  const nodeMap = new Map<string, IndexTreeNode>();
  nodeMap.set("", root);

  const getOrCreateNode = (folderPath: string[]): IndexTreeNode => {
    const key = folderPath.join("/");
    if (nodeMap.has(key)) return nodeMap.get(key)!;
    const parentPath = folderPath.slice(0, -1);
    const parent = parentPath.length === 0 ? root : getOrCreateNode(parentPath);
    const meta = folderMeta.get(key);
    const node: IndexTreeNode = {
      title: meta?.title || folderPath.at(-1) || "",
      path: folderPath,
      children: [],
      folders: [],
    };
    parent.folders = parent.folders || [];
    parent.folders.push(node);
    nodeMap.set(key, node);
    return node;
  };

  for (const note of flat) {
    if (note.isFolderIndex) continue;
    const segments = note.relativeKey.split("/").filter(Boolean);
    const folderSegments = segments.slice(0, -1);
    const node = getOrCreateNode(folderSegments);
    node.children.push({ title: note.title, slug: note.slug });
  }

  return [root];
}

function findFolderNodeBySlugPath(tree: IndexTreeNode[], folderMeta: Map<string, FolderMeta>, slugPath: string[]) {
  const rootSlug = slugPath[0];
  const rootNode = tree.find((node) => node.path.length === 0) || null;
  const levelOne = rootNode?.folders || tree;
  let current: IndexTreeNode | undefined = levelOne.find((node) => {
    const slugs = resolveFolderSlugs(node.path, folderMeta);
    return slugs ? slugs[slugs.length - 1] === rootSlug : false;
  });
  if (!current) return null;

  for (let i = 1; i < slugPath.length; i++) {
    const slug = slugPath[i];
    const next: IndexTreeNode | undefined = (current.folders || []).find((folder) => {
      const slugs = resolveFolderSlugs(folder.path, folderMeta);
      return slugs ? slugs[slugs.length - 1] === slug : false;
    });
    if (!next) return null;
    current = next;
  }
  return current || null;
}

async function buildFolderMeta(objects: S3ObjectEntry[], prefix?: string) {
  const folderMeta = new Map<string, FolderMeta>();
  for (const entry of objects) {
    if (!entry.key.toLowerCase().endsWith(".json")) continue;
    if (!entry.key.toLowerCase().endsWith("_folder.json")) continue;
    const raw = await fetchObject(entry.key);
    if (raw.status !== 200) continue;
    try {
      const parsed = JSON.parse(raw.body) as FolderMeta;
      const pathKey = safeDecode(stripPrefix(entry.key.replace(/_folder\.json$/i, ""), prefix));
      folderMeta.set(pathKey, {
        ...parsed,
        path: pathKey,
        etag: raw.etag || entry.etag,
        renderVersion: FOLDER_RENDER_VERSION,
      });
    } catch {
      // ignore broken meta
    }
  }
  return folderMeta;
}

function buildSlugFromFolders(folderSlugs: string[], baseSlug: string) {
  return [...folderSlugs, baseSlug].join("/");
}

function buildBreadcrumbs(relativeKey: string, folderMeta: Map<string, FolderMeta>, slug: string, title: string, prefix?: string) {
  const segments = relativeKey.split("/").filter(Boolean);
  const folders = segments.slice(0, -1);
  const breadcrumbs: { title: string; href: string }[] = [];
  for (let i = 0; i < folders.length; i++) {
    const pathPart = folders.slice(0, i + 1);
    const metaKey = pathPart.join("/");
    const folderSlugs = resolveFolderSlugs(pathPart, folderMeta);
    const label = folderMeta.get(metaKey)?.title || pathPart[pathPart.length - 1];
    if (folderSlugs) {
      const href = `/folders/${folderSlugs.join("/")}`;
      breadcrumbs.push({ title: label || "Папка", href });
    } else {
      breadcrumbs.push({ title: label || "Папка", href: "#" });
    }
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

function isFolderIndex(key: string) {
  return key.toLowerCase().endsWith("_folder.json");
}
