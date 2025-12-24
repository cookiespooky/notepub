const DEFAULT_SITES_BASE_DOMAIN = "notepub.site";
const DEFAULT_APP_URL = "https://notepub.site";

export function getSitesBaseDomain(): string {
  return process.env.NEXT_PUBLIC_SITES_BASE_DOMAIN || process.env.SITES_BASE_DOMAIN || DEFAULT_SITES_BASE_DOMAIN;
}

export function buildSiteHost(slug: string): string {
  const base = getSitesBaseDomain();
  return `${slug}.${base}`;
}

export function buildSiteUrl(slug: string): string {
  return `https://${buildSiteHost(slug)}`;
}

export function getAppUrl(): string {
  return process.env.NEXT_PUBLIC_APP_URL || process.env.APP_URL || DEFAULT_APP_URL;
}
