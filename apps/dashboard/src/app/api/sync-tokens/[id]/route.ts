import { NextResponse } from "next/server";
import { prisma } from "@notepub/db";
import { setSyncTokenActive } from "@notepub/core";
import { getCurrentUser } from "@/lib/session";

export async function PATCH(request: Request, { params }: { params: { id: string } }) {
  const user = await getCurrentUser();
  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const body = (await request.json().catch(() => ({}))) as { isActive?: boolean; siteId?: string };
  if (!body.siteId) return NextResponse.json({ error: "siteId required" }, { status: 400 });
  const site = await prisma.site.findUnique({ where: { id: body.siteId } });
  if (!site || site.ownerId !== user.id) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  if (typeof body.isActive !== "boolean") {
    return NextResponse.json({ error: "isActive required" }, { status: 400 });
  }
  try {
    await setSyncTokenActive(params.id, user.id, body.isActive, body.siteId);
    return NextResponse.json({ ok: true });
  } catch (error) {
    return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  }
}
