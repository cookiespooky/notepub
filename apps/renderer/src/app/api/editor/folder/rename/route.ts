import { NextRequest, NextResponse } from "next/server";
import { listRawNotesWithDrafts } from "@/lib/editorNotes";
import { copyObject, deleteObject, listFolderPlaceholders, putObject } from "@/lib/s3";
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

  const fromRaw = normalizeFolder(payload?.from);
  const toRaw = normalizeFolder(payload?.to);
  if (!fromRaw || !toRaw) {
    return NextResponse.json({ error: "Invalid folder name" }, { status: 400 });
  }
  if (fromRaw === toRaw) {
    return NextResponse.json({ error: "Same folder" }, { status: 400 });
  }

  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const placeholders = await listFolderPlaceholders(context.prefix || undefined);
  const fromPlaceholder = `${fromRaw}.keep`;
  const toPlaceholder = `${toRaw}.keep`;

  // Build list of objects to move
  const movingNotes = notes.filter((note) => normalizePath(note.relativeKey).startsWith(fromRaw));
  const otherNotes = notes.filter((note) => !normalizePath(note.relativeKey).startsWith(fromRaw));

  // Conflict check
  for (const note of movingNotes) {
    const rel = normalizePath(note.relativeKey);
    const rest = rel.slice(fromRaw.length);
    const target = `${toRaw}${rest}`;
    if (otherNotes.some((n) => normalizePath(n.relativeKey) === target)) {
      return NextResponse.json({ error: "Target path already exists" }, { status: 409 });
    }
  }

  // Move notes
  for (const note of movingNotes) {
    const rel = normalizePath(note.relativeKey);
    const rest = rel.slice(fromRaw.length);
    const target = `${toRaw}${rest}`;
    await copyObject(rel, target, context.prefix || undefined);
  }
  for (const note of movingNotes) {
    const rel = normalizePath(note.relativeKey);
    await deleteObject(rel, context.prefix || undefined).catch(() => {});
  }

  // Move placeholder
  const hasPlaceholder = placeholders.some((p) => normalizePath(p.key) === fromPlaceholder);
  if (hasPlaceholder) {
    await copyObject(fromPlaceholder, toPlaceholder, context.prefix || undefined);
    await deleteObject(fromPlaceholder, context.prefix || undefined).catch(() => {});
  } else {
    await putObject(toPlaceholder, "", context.prefix || undefined).catch(() => {});
  }

  return NextResponse.json({ ok: true, from: fromRaw, to: toRaw });
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
