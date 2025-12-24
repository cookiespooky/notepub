import Link from "next/link";
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { getSiteBySlug } from "@notepub/core";
import { listObjects } from "@notepub/storage";

export const dynamic = "force-dynamic";

export default async function MapPage({ searchParams }: { searchParams?: { slug?: string } }) {
  const host = headers().get("host") || "";
  const siteSlug = searchParams?.slug || headers().get("x-site-slug") || extractSlugFromHost(host);
  if (!siteSlug) notFound();

  const site = await getSiteBySlug(siteSlug);
  if (!site) notFound();

  const objects = await listObjects(site.s3Prefix);
  const entries = objects
    .filter((obj) => obj.key.toLowerCase().endsWith(".md"))
    .map((obj) => ({
      key: obj.key,
      relative: stripPrefix(obj.key, site.s3Prefix),
    }))
    .sort((a, b) => a.relative.localeCompare(b.relative));

  return (
    <main style={{ padding: "32px 20px", maxWidth: 960, margin: "0 auto" }}>
      <h1 style={{ marginBottom: 12 }}>Site map</h1>
      <p style={{ marginBottom: 16 }}>Listing markdown files under {site.s3Prefix}</p>
      <ul style={{ listStyle: "none", padding: 0, margin: 0, display: "grid", gap: 8 }}>
        {entries.map((entry) => {
          const href = toSlugPath(entry.relative);
          return (
            <li key={entry.key}>
              <Link href={`/${href}`} style={{ color: "#0c4a6e" }}>
                {href}
              </Link>
            </li>
          );
        })}
      </ul>
    </main>
  );
}

function stripPrefix(key: string, prefix: string) {
  const normalizedPrefix = prefix.replace(/^\/+/, "").replace(/\/+$/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedPrefix && normalizedKey.startsWith(normalizedPrefix + "/")) {
    return normalizedKey.slice(normalizedPrefix.length + 1);
  }
  return normalizedKey;
}

function toSlugPath(relative: string) {
  return relative.replace(/\.md$/i, "");
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
