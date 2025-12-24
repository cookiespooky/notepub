import { NextRequest, NextResponse } from "next/server";
import matter from "gray-matter";
import { listRawNotesWithDrafts } from "@/lib/editorNotes";
import { requireSiteFromRequest } from "@/lib/siteContext";

export async function GET(req: NextRequest) {
  const context = await requireSiteFromRequest(req);
  if (!context) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  const { searchParams } = new URL(req.url);
  const slug = searchParams.get("slug");
  const path = searchParams.get("path");
  if (!slug && !path) {
    return NextResponse.json({ error: "Missing slug or path" }, { status: 400 });
  }
  const notes = await listRawNotesWithDrafts(context.prefix || undefined);
  const note = notes.find((n) => (slug ? n.slug === slug : n.relativeKey === path));
  if (!note) {
    return NextResponse.json({ error: "Not found" }, { status: 404 });
  }

  const parsed = matter.stringify(note.content, note.frontmatter);

  return NextResponse.json({
    path: note.relativeKey,
    slug: note.slug,
    title: note.title,
    category: note.category,
    home: note.isHome,
    draft: note.isDraft,
    frontmatter: note.frontmatter,
    body: note.content,
    raw: parsed,
  });
}
