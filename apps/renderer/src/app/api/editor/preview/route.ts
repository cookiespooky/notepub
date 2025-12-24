import { NextRequest, NextResponse } from "next/server";
import { markdownToHtml } from "@/lib/markdown";
import { listRawNotesWithDrafts } from "@/lib/editorNotes";
import { requireSiteFromRequest } from "@/lib/siteContext";
import { buildSlugLookup } from "@/lib/notes";

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
  const { body, objectKey } = payload || {};
  if (typeof body !== "string") {
    return NextResponse.json({ error: "Missing body" }, { status: 400 });
  }
  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const slugLookup = buildSlugLookup(notes);
  const html = await markdownToHtml(body, {
    objectKey: objectKey || "",
    s3Prefix: context.prefix || undefined,
    slugLookup,
  });
  return NextResponse.json({ html });
}
