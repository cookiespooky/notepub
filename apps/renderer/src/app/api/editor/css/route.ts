import { NextRequest, NextResponse } from "next/server";
import { requireSiteFromRequest } from "@/lib/siteContext";
import { fetchObject, putObject } from "@/lib/s3";

const THEME_KEY = "theme.json";

export async function GET(req: NextRequest) {
  const context = await requireSiteFromRequest(req);
  if (!context) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const response = await fetchObject(buildKey(context.prefix));
    if (response.status === 200 && response.body) {
      try {
        const parsed = JSON.parse(response.body);
        return NextResponse.json({ settings: parsed || {} });
      } catch {
        return NextResponse.json({ settings: {} });
      }
    }
    return NextResponse.json({ settings: {} });
  } catch (error: unknown) {
    return NextResponse.json({ settings: {} });
  }
}

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

  const settings = payload?.settings;
  if (!settings || typeof settings !== "object") {
    return NextResponse.json({ error: "Invalid settings" }, { status: 400 });
  }

  await putObject(buildKey(context.prefix), JSON.stringify(settings, null, 2));
  return NextResponse.json({ ok: true });
}

function buildKey(prefix?: string | null) {
  const clean = (prefix || "").replace(/^\/+|\/+$/g, "");
  return clean ? `${clean}/${THEME_KEY}` : THEME_KEY;
}
