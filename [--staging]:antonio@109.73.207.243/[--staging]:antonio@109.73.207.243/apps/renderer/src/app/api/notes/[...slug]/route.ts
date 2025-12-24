import { NextResponse } from "next/server";
import { getNoteBySlug } from "@/lib/notes";

export const dynamic = "force-dynamic";

export async function GET(_: Request, { params }: { params: { slug: string[] } }) {
  const slug = Array.isArray(params.slug) ? params.slug.join("/") : params.slug;
  if (!slug) return NextResponse.json({ error: "Slug required" }, { status: 400 });

  try {
    const note = await getNoteBySlug(decodeURIComponent(slug));
    if (!note) {
      return NextResponse.json({ error: "Not found" }, { status: 404 });
    }
    return NextResponse.json(note);
  } catch (error: unknown) {
    console.error("Failed to load note", slug, error);
    return NextResponse.json({ error: "Failed to load note" }, { status: 500 });
  }
}
