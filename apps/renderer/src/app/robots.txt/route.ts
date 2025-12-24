import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { headers } from "next/headers";
import { getSiteBySlug } from "@notepub/core";
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

  const fallbackHost = buildSiteHost(site.slug);
  const sitemapUrl = `${proto}://${host || fallbackHost}/sitemap.xml`;
  const body = renderRobots(sitemapUrl);

  return new NextResponse(body, {
    status: 200,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "public, max-age=600",
    },
  });
}

function renderRobots(sitemapUrl: string) {
  return [
    "User-agent: *",
    "Allow: /",
    "User-agent: GPTBot",
    "Allow: /",
    "User-agent: Google-Extended",
    "Allow: /",
    "User-agent: oai-crawler",
    "Allow: /",
    "User-agent: anthropic-ai",
    "Allow: /",
    `Sitemap: ${sitemapUrl}`,
    "",
  ].join("\n");
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
