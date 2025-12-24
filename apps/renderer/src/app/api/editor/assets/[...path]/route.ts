import { NextRequest, NextResponse } from "next/server";
import { getSignedObjectUrlWithPrefix } from "@/lib/s3";
import { getSiteFromRequestAllowAnonymous } from "@/lib/siteContext";

export async function GET(
  req: NextRequest,
  context: { params: { path: string[] } },
) {
  const site = await getSiteFromRequestAllowAnonymous(req);
  if (!site) {
    return NextResponse.json({ error: "Unknown site" }, { status: 400 });
  }
  const pathParts = context.params.path || [];
  const rawKey = pathParts.join("/");
  if (!rawKey) {
    return NextResponse.json({ error: "Invalid asset path" }, { status: 400 });
  }
  const key = rawKey.replace(/\\/g, "/").replace(/^\/+/, "");
  const url = await getSignedObjectUrlWithPrefix(key, site.prefix || undefined, 3600);
  return NextResponse.redirect(url, { status: 302 });
}
