import { NextResponse } from "next/server";
import { createSyncToken, listSyncTokens } from "@notepub/core";
import { prisma } from "@notepub/db";
import { getCurrentUser } from "@/lib/session";

export async function GET(request: Request) {
  const user = await getCurrentUser();
  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const { searchParams } = new URL(request.url);
  const siteId = searchParams.get("siteId");
  if (!siteId) return NextResponse.json({ error: "siteId required" }, { status: 400 });
  const site = await prisma.site.findUnique({ where: { id: siteId } });
  if (!site || site.ownerId !== user.id) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const tokens = await listSyncTokens(user.id, siteId);
  return NextResponse.json({ tokens });
}

export async function POST(request: Request) {
  const user = await getCurrentUser();
  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const body = (await request.json().catch(() => ({}))) as { label?: string; siteId?: string };
  if (!body.siteId) return NextResponse.json({ error: "siteId required" }, { status: 400 });
  const site = await prisma.site.findUnique({ where: { id: body.siteId } });
  if (!site || site.ownerId !== user.id) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const { token, record } = await createSyncToken(user.id, body.siteId, body.label);
  return NextResponse.json({ token, tokenId: record.id, label: record.label, createdAt: record.createdAt });
}
