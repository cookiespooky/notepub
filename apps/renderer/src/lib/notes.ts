import crypto from "crypto";
import matter from "gray-matter";
import path from "path";
import { markdownToHtml } from "./markdown";
import { fetchObject, listNoteObjects } from "./s3";
import { getS3Config } from "./config";
import { resolveFolderSlugs, slugFromPathSegments, slugifySegment } from "./slug";
import { FlatNoteIndex, FolderListing, FolderMeta, IndexData, IndexTreeNode, NoteResponse, S3ObjectEntry } from "./types";

type NoteCache = NoteResponse & {
  key: string;
  relativeKey: string;
  etag: string;
  renderVersion: string;
};

const NOTE_RENDER_VERSION = "19-folder-pages";
const INDEX_RENDER_VERSION = "19-folder-pages";
const FOLDER_RENDER_VERSION = "9-folder-raw-slug";

function hashList(objects: S3ObjectEntry[]) {
  const sorted = [...objects].sort((a, b) => a.key.localeCompare(b.key));
  const hash = crypto.createHash("md5");
  for (const entry of sorted) {
    hash.update(entry.key);
    hash.update(entry.etag);
    hash.update(entry.lastModified || "");
  }
  return hash.digest("hex");
}

export async function getIndexData(prefix?: string): Promise<IndexData> {
  const objects = await listNoteObjects(prefix);
  const listEtag = hashList(objects);

  const folderMeta = await buildFolderMeta(objects, prefix);

  const flat: FlatNoteIndex[] = [];
  for (const entry of objects) {
    if (isFolderIndex(entry.key)) continue;
    const note = await loadNote(entry, folderMeta, undefined, prefix);
    flat.push({
      key: note.key,
      relativeKey: note.relativeKey,
      title: note.title,
      slug: note.slug,
      tags: note.tags,
      html: note.html,
    });
  }

  const tree = buildTree(flat, folderMeta);
  return { tree, flat, folderMeta };
}

export async function getNoteBySlug(slug: string, prefix?: string): Promise<NoteResponse | null> {
  const index = await getIndexData(prefix);
  const entry = index.flat.find((item) => item.slug === slug);
  if (!entry) return null;
  const note = await loadNote(
    {
      key: entry.key,
      etag: entry.etag || "",
      lastModified: null,
    },
    index.folderMeta,
    entry.slug,
    prefix,
  );
  return note;
}

export async function getFolderBySlugPath(slugPath: string[], prefix?: string): Promise<FolderListing | null> {
  const index = await getIndexData(prefix);
  const target = findFolderNodeBySlugPath(index.tree, index.folderMeta, slugPath);
  if (!target) return null;

  const folderSlugs = resolveFolderSlugs(target.path, index.folderMeta);
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
    folders: (target.folders || []).map((f) => ({
      title: f.title,
      slugPath: resolveFolderSlugs(f.path, index.folderMeta),
    })),
    notes: target.children.map((n) => ({ title: n.title, slug: n.slug })),
  };
}

async function loadNote(entry: S3ObjectEntry, folderMeta: Map<string, FolderMeta>, slugOverride?: string, prefix?: string): Promise<NoteCache> {
  const remote = await fetchObject(entry.key);

  const markdown = remote.body;
  const parsed = matter(markdown);
  const frontmatter = (parsed.data ?? {}) as Record<string, unknown>;
  const relativeKey = stripPrefix(entry.key, prefix);
  const title = typeof frontmatter.title === "string" ? frontmatter.title : deriveTitle(relativeKey);
  const fmSlug = typeof frontmatter.slug === "string" ? frontmatter.slug.trim() : null;
  const baseName = path.posix.basename(relativeKey, path.extname(relativeKey));
  const slug =
    slugOverride || buildSlugFromFolders(relativeKey, fmSlug || baseName, folderMeta, Boolean(fmSlug));
  const tags = normalizeTags(frontmatter.tags);
  const created = frontmatter.created ? String(frontmatter.created) : null;
  const updated = frontmatter.updated ? String(frontmatter.updated) : remote.lastModified || entry.lastModified || null;
  const html = await markdownToHtml(parsed.content, { objectKey: entry.key, folderMeta });
  const breadcrumbs = buildBreadcrumbs(relativeKey, folderMeta, slug, title, prefix);
  const payload: NoteCache = {
    key: entry.key,
    relativeKey,
    etag: remote.etag || entry.etag || "",
    slug,
    title,
    tags,
    html,
    created,
    updated,
    breadcrumbs,
    renderVersion: NOTE_RENDER_VERSION,
  };
  return payload;
}

function deriveTitle(key: string) {
  const base = path.posix.basename(key, path.extname(key));
  return unslugify(base);
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
    return slugs[slugs.length - 1] === rootSlug;
  });
  if (!current) return null;

  for (let i = 1; i < slugPath.length; i++) {
    const slug = slugPath[i];
    const next: IndexTreeNode | undefined = (current.folders || []).find((folder) => {
      const slugs = resolveFolderSlugs(folder.path, folderMeta);
      return slugs[slugs.length - 1] === slug;
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
      folderMeta.set(stripPrefix(entry.key.replace(/_folder\.json$/i, ""), prefix), {
        ...parsed,
        path: stripPrefix(entry.key.replace(/_folder\.json$/i, ""), prefix),
        etag: raw.etag || entry.etag,
        renderVersion: FOLDER_RENDER_VERSION,
      });
    } catch {
      // ignore broken meta
    }
  }
  return folderMeta;
}

function buildSlugFromFolders(relativeKey: string, baseSlug: string, folderMeta: Map<string, FolderMeta>, hasFrontmatterSlug: boolean) {
  const segments = relativeKey.split("/").filter(Boolean);
  const folderSegments = segments.slice(0, -1);
  const folderSlugs = resolveFolderSlugs(folderSegments, folderMeta);
  const base = hasFrontmatterSlug ? slugifySegment(baseSlug) : baseSlug;
  return slugFromPathSegments([...folderSlugs, base]);
}

function buildBreadcrumbs(relativeKey: string, folderMeta: Map<string, FolderMeta>, slug: string, title: string, prefix?: string) {
  const segments = relativeKey.split("/").filter(Boolean);
  const folders = segments.slice(0, -1);
  const breadcrumbs: { title: string; href: string }[] = [];
  for (let i = 0; i < folders.length; i++) {
    const pathPart = folders.slice(0, i + 1);
    const metaKey = pathPart.join("/");
    const href = `/folders/${resolveFolderSlugs(pathPart, folderMeta).join("/")}`;
    const label = folderMeta.get(metaKey)?.title || pathPart[pathPart.length - 1];
    breadcrumbs.push({ title: label || "Папка", href });
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

function unslugify(input: string) {
  return input
    .replace(/[-_]/g, " ")
    .split(" ")
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function isFolderIndex(key: string) {
  return key.toLowerCase().endsWith("_folder.json");
}
