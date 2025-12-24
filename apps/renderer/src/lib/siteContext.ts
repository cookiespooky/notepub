import { NextRequest } from "next/server";
import { getSiteBySlug } from "@notepub/core";
import { getCurrentUser } from "./session";

type SiteContext = {
  site: Awaited<ReturnType<typeof getSiteBySlug>>;
  prefix: string | null;
};

export async function getSiteFromRequestAllowAnonymous(req: NextRequest): Promise<SiteContext | null> {
  const host = req.headers.get("host") || "";
  const siteSlug = req.headers.get("x-site-slug") || extractSlugFromHost(host);
  if (!siteSlug) return null;
  const site = await getSiteBySlug(siteSlug);
  if (!site) return null;
  return { site, prefix: (site as any).s3Prefix || null };
}

export async function requireSiteFromRequest(req: NextRequest): Promise<SiteContext | null> {
  const host = req.headers.get("host") || "";
  const siteSlug = req.headers.get("x-site-slug") || extractSlugFromHost(host);
  if (!siteSlug) return null;
  const site = await getSiteBySlug(siteSlug);
  if (!site) return null;
  const user = await getCurrentUser();
  if (!user || user.id !== site.ownerId) return null;
  return { site, prefix: (site as any).s3Prefix || null };
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
