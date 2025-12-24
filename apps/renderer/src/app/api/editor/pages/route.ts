import { NextRequest, NextResponse } from "next/server";
import { listFolders, listRawNotesWithDrafts } from "@/lib/editorNotes";
import { requireSiteFromRequest } from "@/lib/siteContext";

export async function GET(req: NextRequest) {
  const context = await requireSiteFromRequest(req);
  if (!context) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const folders = await listFolders(context.prefix || undefined);
  const payload = notes.map((note) => ({
    path: note.relativeKey,
    slug: note.slug,
    title: note.title,
    category: note.category,
    home: note.isHome,
    draft: note.isDraft,
  }));

  return NextResponse.json({ pages: payload, folders });
}
