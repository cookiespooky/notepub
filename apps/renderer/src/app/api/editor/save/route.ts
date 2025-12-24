import { NextRequest, NextResponse } from "next/server";
import { listRawNotesWithDrafts, saveNote, slugExists } from "@/lib/editorNotes";
import { requireSiteFromRequest } from "@/lib/siteContext";

export async function POST(req: NextRequest) {
  const context = await requireSiteFromRequest(req);
  if (!context) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  let payload: any;
  try {
    payload = await req.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });
  }
  const { frontmatter, body, path, targetPath } = payload || {};
  if (
    !frontmatter ||
    typeof body !== "string" ||
    typeof frontmatter.title !== "string" ||
    frontmatter.title.trim().length === 0
  ) {
    return NextResponse.json({ error: "Missing title" }, { status: 400 });
  }

  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const resolvedPath = normalizePath(targetPath || path) || ensureRootFilename(frontmatter.title || "");
  const normalizedSlug = typeof frontmatter.slug === "string" ? frontmatter.slug.trim() : "";
  if (normalizedSlug && slugExists(notes, normalizedSlug, path || resolvedPath)) {
    return NextResponse.json({ error: "Slug already exists" }, { status: 409 });
  }
  if (pathConflict(notes, resolvedPath, path)) {
    return NextResponse.json({ error: "Path already exists" }, { status: 409 });
  }

  const result = await saveNote({
    frontmatter,
    body,
    path,
    prefix: context.prefix || undefined,
    targetPath: resolvedPath,
  });

  return NextResponse.json(result);
}

function normalizePath(input?: string | null) {
  if (!input) return "";
  const trimmed = input.replace(/^\/+/, "");
  if (!trimmed) return "";
  return trimmed.endsWith(".md") ? trimmed : `${trimmed}.md`;
}

function ensureRootFilename(input: string) {
  const base = (input || "").trim() || "untitled";
  const withoutSlashes = base.replace(/[\\/]+/g, "-");
  const sanitized = withoutSlashes.replace(/^\.+/, "");
  const name = sanitized || "untitled";
  return name.endsWith(".md") ? name : `${name}.md`;
}

function pathConflict(notes: Awaited<ReturnType<typeof listRawNotesWithDrafts>>, candidate: string, exclude?: string) {
  const normalizedCandidate = normalizePath(candidate);
  const normalizedExclude = normalizePath(exclude || "");
  if (!normalizedCandidate) return false;
  return notes.some((note) => {
    const rel = normalizePath(note.relativeKey);
    const full = normalizePath(note.entry.key);
    if (rel === normalizedExclude || full === normalizedExclude) return false;
    return rel === normalizedCandidate || full === normalizedCandidate;
  });
}
