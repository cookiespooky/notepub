import { NextResponse } from "next/server";
import { prisma } from "@notepub/db";

export async function POST(request: Request) {
  try {
    const body = await request.json().catch(() => ({}));
    const rawEmail = (body.email || "").toString().trim().toLowerCase();
    if (!rawEmail) {
      return NextResponse.json({ error: "Email обязателен" }, { status: 400 });
    }
    const user = await prisma.user.findUnique({ where: { email: rawEmail }, select: { id: true } });
    const isProd = process.env.APP_ENV === "production" || process.env.NODE_ENV === "production";
    let debug: Record<string, string> | undefined;
    if (!isProd && process.env.DATABASE_URL) {
      try {
        const url = new URL(process.env.DATABASE_URL);
        debug = { dbHost: url.hostname, dbName: url.pathname.replace(/^\//, "") };
      } catch {
        // ignore parse errors
      }
    }
    if (!isProd) {
      console.log("[check-email]", { email: rawEmail, exists: Boolean(user), debug });
    }
    return NextResponse.json({ exists: Boolean(user), debug });
  } catch (error) {
    console.error("check-email error", error);
    return NextResponse.json({ error: "Server error" }, { status: 500 });
  }
}
