import { NextRequest, NextResponse } from "next/server";
import { requireSiteFromRequest } from "@/lib/siteContext";
import { deleteObject } from "@/lib/s3";
import { getRawNotes } from "@/lib/notes";

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
  const url = (payload?.url || "").toString().trim();
  const force = Boolean(payload?.force);
  if (!url) return NextResponse.json({ error: "Missing url" }, { status: 400 });

  const key = extractAssetKey(url);
  if (!key) {
    return NextResponse.json({ error: "Invalid asset url" }, { status: 400 });
  }

  if (!force) {
    // Reference check across notes (including drafts)
    const notes = await getRawNotes(context.prefix || undefined, { includeDrafts: true });
    const matches = notes.some((note) => containsAsset(note.content, key));
    if (matches) {
      return NextResponse.json({ error: "Asset is still referenced" }, { status: 409 });
    }
  }

  await deleteObject(key, context.prefix || undefined).catch(() => {});
  return NextResponse.json({ ok: true });
}

function extractAssetKey(url: string) {
  // Accept /api/editor/assets/... or raw .np-assets/... or filename-only
  const cleaned = url.replace(/\?.*$/, "");
  const apiPrefix = "/api/editor/assets/";
  let key = cleaned;
  if (cleaned.startsWith(apiPrefix)) {
    key = cleaned.slice(apiPrefix.length);
  }
  key = key.replace(/^\/+/, "");
  return key || null;
}

function containsAsset(content: string, key: string) {
  const normalized = key.startsWith("/") ? key.slice(1) : key;
  const candidates = [
    key,
    `./${key}`,
    normalized,
    `/api/editor/assets/${key}`,
    `/api/editor/assets/${normalized}`,
  ];
  return candidates.some((c) => content.includes(c));
}
