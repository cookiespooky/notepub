import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { headers } from "next/headers";
import { getSiteBySlug } from "@notepub/core";
import { getIndexData } from "@/lib/notes";
import { buildSiteHost } from "@/lib/domains";

export const dynamic = "force-dynamic";

export async function GET(_req: NextRequest) {
  const host = headers().get("host") || "";
  const proto = headers().get("x-forwarded-proto") || "https";
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(host);
  if (!siteSlug) {
    return NextResponse.json({ error: "site not found" }, { status: 404 });
  }

  const site = await getSiteBySlug(siteSlug);
  if (!site) {
    return NextResponse.json({ error: "site not found" }, { status: 404 });
  }

  const index = await getIndexData(site.s3Prefix);
  const origin = `${proto}://${host || buildSiteHost(site.slug)}`;

  const urls = new Set<string>();

  urls.add(new URL("/", origin).toString());
  for (const entry of index.flat) {
    const loc = entry.isHome ? "/" : `/${entry.slug}`;
    urls.add(new URL(loc, origin).toString());
  }

  const body = renderSitemap([...urls]);
  return new NextResponse(body, {
    status: 200,
    headers: {
      "Content-Type": "application/xml; charset=utf-8",
      "Cache-Control": "public, max-age=600",
    },
  });
}

function renderSitemap(urls: string[]) {
  const items = urls
    .sort()
    .map((loc) => `<url><loc>${escapeXml(loc)}</loc></url>`)
    .join("");
  return `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${items}</urlset>`;
}

function escapeXml(value: string) {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function extractSlugFromHost(host: string) {
  const withoutPort = host.split(":")[0] || "";
  const parts = withoutPort.split(".").filter(Boolean);
  if (parts.length === 2 && parts[1] === "localhost") {
    return parts[0];
  }
  if (parts.length <= 2) return "";
  return parts[0];
}
