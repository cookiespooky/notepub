import Link from "next/link";
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { getSiteBySlug } from "@notepub/core";
import { getIndexData } from "@/lib/notes";

export const dynamic = "force-dynamic";

export default async function MapPage({ searchParams }: { searchParams?: { slug?: string } }) {
  const host = headers().get("host") || "";
  const siteSlug = searchParams?.slug || headers().get("x-site-slug") || extractSlugFromHost(host);
  if (!siteSlug) notFound();

  const site = await getSiteBySlug(siteSlug);
  if (!site) notFound();

  const index = await getIndexData(site.s3Prefix, { includeDrafts: true });
  const entries = index.flat
    .map((note) => ({
      slug: note.slug,
      title: note.title,
      category: note.category,
    }))
    .sort((a, b) => a.slug.localeCompare(b.slug, "ru"));

  return (
    <main style={{ padding: "32px 20px", maxWidth: 960, margin: "0 auto" }}>
      <h1 style={{ marginBottom: 12 }}>Site map</h1>
      <p style={{ marginBottom: 16 }}>Listing published note slugs</p>
      <ul style={{ listStyle: "none", padding: 0, margin: 0, display: "grid", gap: 8 }}>
        {entries.map((entry) => (
          <li key={entry.slug}>
            <Link href={`/${entry.slug}`} style={{ color: "#0c4a6e" }}>
              {entry.slug}
            </Link>
            <span style={{ color: "#475569", marginLeft: 8 }}>
              {entry.category ? `(${entry.category})` : "(без категории)"} — {entry.title}
            </span>
          </li>
        ))}
      </ul>
    </main>
  );
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
