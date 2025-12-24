import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { getSiteBySlug } from "@notepub/core";
import type { Metadata } from "next";
import { Sidebar } from "@/components/Sidebar";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { getCurrentUser } from "@/lib/session";
import { getIndexData } from "@/lib/notes";
import { buildSiteUrl } from "@/lib/domains";
import styles from "./page.module.css";

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: { params: { slug: string } }): Promise<Metadata> {
  const hostHeader = headers().get("host") || "";
  const proto = headers().get("x-forwarded-proto") || (hostHeader.includes("localhost") ? "http" : "https");
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(hostHeader);
  if (!siteSlug) return {};

  const site = await getSiteBySlug(siteSlug);
  if (!site) return {};

  const currentUser = await getCurrentUser();
  const isOwner = currentUser?.id === site.ownerId;
  const index = await getIndexData(site.s3Prefix, { includeDrafts: isOwner });
  const categorySlug = decodeURIComponent(params.slug);
  const category = index.categories.find((cat) => cat.slug === categorySlug);
  if (!category) return {};

  const origin = hostHeader ? `${proto}://${hostHeader}` : buildSiteUrl(site.slug, proto);
  const canonical = `${origin}/category/${category.slug}`;
  const title = `${category.name} — ${site.title}`;

  return {
    title,
    description: site.title,
    alternates: { canonical },
  };
}

export default async function CategoryPage({ params }: { params: { slug: string } }) {
  const hostHeader = headers().get("host") || "";
  const proto = headers().get("x-forwarded-proto") || (hostHeader.includes("localhost") ? "http" : "https");
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(hostHeader);
  if (!siteSlug) notFound();

  const site = await getSiteBySlug(siteSlug);
  if (!site) notFound();

  const currentUser = await getCurrentUser();
  const isOwner = currentUser?.id === site.ownerId;

  const index = await getIndexData(site.s3Prefix, { includeDrafts: isOwner });
  const categorySlug = decodeURIComponent(params.slug);
  const category = index.categories.find((cat) => cat.slug === categorySlug);
  if (!category) notFound();

  const flatForClient = index.flat.map(({ key: _key, ...rest }) => rest);
  const breadcrumbs = [{ title: category.name, href: null }];
  const origin = hostHeader ? `${proto}://${hostHeader}` : buildSiteUrl(site.slug, proto);
  const canonical = `${origin}/category/${category.slug}`;

  return (
    <div className="page-shell">
      <Sidebar
        siteSlug={site.slug}
        categories={index.categories}
        flat={flatForClient}
        activeSlug=""
        activeCategorySlug={category.slug}
        siteTitle={site.title}
        siteAvatarUrl={site.ogImageUrl}
      />
      <main className="page-main">
        <div className='page-content'>
          <div className={styles.header}>
          <Breadcrumbs crumbs={breadcrumbs} />
          </div>
          <div className={styles.list}>
            {category.notes.map((note) => (
              <a key={note.slug} href={`/${note.slug}`} className={styles.item}>
                <div className={styles.noteTitle}>{note.title}</div>
                {note.isDraft && isOwner && <span className={styles.draft}>Черновик</span>}
              </a>
            ))}
          </div>
          <link rel="canonical" href={canonical} />
        </div>
      </main>
    </div>
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
