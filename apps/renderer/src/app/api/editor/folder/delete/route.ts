import { NextRequest, NextResponse } from "next/server";
import { listRawNotesWithDrafts } from "@/lib/editorNotes";
import { deleteObject, listFolderPlaceholders } from "@/lib/s3";
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

  const folder = normalizeFolder(payload?.path);
  if (!folder) {
    return NextResponse.json({ error: "Invalid folder name" }, { status: 400 });
  }

  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const placeholders = await listFolderPlaceholders(context.prefix || undefined);

  const affected = notes.filter((note) => normalizePath(note.relativeKey).startsWith(folder));

  for (const note of affected) {
    const rel = normalizePath(note.relativeKey);
    await deleteObject(rel, context.prefix || undefined).catch(() => {});
  }

  const placeholderKey = `${folder}.keep`;
  const hasPlaceholder = placeholders.some((p) => normalizePath(p.key) === placeholderKey);
  if (hasPlaceholder) {
    await deleteObject(placeholderKey, context.prefix || undefined).catch(() => {});
  }

  return NextResponse.json({ ok: true, deleted: affected.length });
}

function normalizeFolder(input?: string | null) {
  if (!input || typeof input !== "string") return "";
  const trimmed = input.replace(/^[\\/]+|[\\/]+$/g, "").trim();
  if (!trimmed || trimmed.includes("..")) return "";
  return `${trimmed}/`;
}

function normalizePath(input?: string | null) {
  if (!input) return "";
  return input.replace(/^\/+/, "");
}
