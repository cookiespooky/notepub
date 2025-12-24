import { NextRequest, NextResponse } from "next/server";
import { listRawNotesWithDrafts } from "@/lib/editorNotes";
import { putObject } from "@/lib/s3";
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

  const { path } = payload || {};
  const folder = normalizeFolder(path);
  if (!folder) {
    return NextResponse.json({ error: "Invalid folder name" }, { status: 400 });
  }

  const placeholder = `${folder}.keep`;
  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  if (pathConflict(notes, placeholder)) {
    return NextResponse.json({ error: "Folder already exists" }, { status: 409 });
  }

  await putObject(placeholder, "", context.prefix || undefined);
  return NextResponse.json({ ok: true, path: placeholder });
}

function normalizeFolder(input?: string | null) {
  if (!input || typeof input !== "string") return "";
  const trimmed = input.replace(/^[\\/]+|[\\/]+$/g, "").trim();
  if (!trimmed) return "";
  if (trimmed.includes("..")) return "";
  return `${trimmed}/`;
}

function normalizePath(input: string) {
  const trimmed = input.replace(/^\/+/, "");
  if (!trimmed) return "";
  return trimmed.endsWith(".md") ? trimmed : `${trimmed}`;
}

function pathConflict(notes: Awaited<ReturnType<typeof listRawNotesWithDrafts>>, candidate: string) {
  const normalizedCandidate = normalizePath(candidate);
  return notes.some((note) => {
    const rel = normalizePath(note.relativeKey);
    const full = normalizePath(note.entry.key);
    return rel === normalizedCandidate || full === normalizedCandidate;
  });
}
