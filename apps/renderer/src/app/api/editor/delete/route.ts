import { NextRequest, NextResponse } from "next/server";
import { listRawNotesWithDrafts } from "@/lib/editorNotes";
import { deleteObject } from "@/lib/s3";
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

  const { path, slug } = payload || {};
  if ((!path || typeof path !== "string") && (!slug || typeof slug !== "string")) {
    return NextResponse.json({ error: "Missing path or slug" }, { status: 400 });
  }

  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const target = findNote(notes, { path, slug });
  if (!target) {
    return NextResponse.json({ error: "Not found" }, { status: 404 });
  }

  await deleteObject(target.relativeKey, context.prefix || undefined);
  return NextResponse.json({ ok: true, path: target.relativeKey, slug: target.slug });
}

function normalizePath(input?: string | null) {
  if (!input) return "";
  const trimmed = input.replace(/^\/+/, "");
  return trimmed.endsWith(".md") ? trimmed : `${trimmed}.md`;
}

function findNote(
  notes: Awaited<ReturnType<typeof listRawNotesWithDrafts>>,
  lookup: { path?: string; slug?: string },
) {
  const normalizedPath = normalizePath(lookup.path);
  if (normalizedPath) {
    const byPath = notes.find((note) => {
      const relative = normalizePath(note.relativeKey);
      const entry = normalizePath(note.entry.key);
      return relative === normalizedPath || entry === normalizedPath;
    });
    if (byPath) return byPath;
  }
  if (lookup.slug) {
    const slug = lookup.slug.trim();
    if (slug) {
      const bySlug = notes.find((note) => note.slug === slug);
      if (bySlug) return bySlug;
    }
  }
  return null;
}
