import { NextRequest, NextResponse } from "next/server";
import { listRawNotesWithDrafts, saveNote, slugExists } from "@/lib/editorNotes";
import { requireSiteFromRequest } from "@/lib/siteContext";

const DEFAULT_BODY = "";

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

  const { frontmatter, body, path } = payload || {};
  if (!frontmatter || typeof frontmatter.title !== "string") {
    return NextResponse.json({ error: "Missing title" }, { status: 400 });
  }

  const fm = { draft: true, ...frontmatter };
  if (!fm.slug || typeof fm.slug !== "string") {
    fm.slug = "";
  }

  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const normalizedSlug = typeof fm.slug === "string" ? fm.slug.trim() : "";
  if (normalizedSlug && slugExists(notes, normalizedSlug as string)) {
    return NextResponse.json({ error: "Slug already exists" }, { status: 409 });
  }

  const targetPath = normalizePath(path) || ensureRootFilename(frontmatter.title || "untitled");
  if (pathConflict(notes, targetPath)) {
    return NextResponse.json({ error: "Path already exists" }, { status: 409 });
  }

  const result = await saveNote({
    frontmatter: fm,
    body: typeof body === "string" ? body : DEFAULT_BODY,
    path: undefined,
    prefix: context.prefix || undefined,
    targetPath,
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

function pathConflict(notes: Awaited<ReturnType<typeof listRawNotesWithDrafts>>, candidate: string) {
  const normalizedCandidate = normalizePath(candidate);
  if (!normalizedCandidate) return false;
  return notes.some((note) => {
    const rel = normalizePath(note.relativeKey);
    const full = normalizePath(note.entry.key);
    return rel === normalizedCandidate || full === normalizedCandidate;
  });
}
