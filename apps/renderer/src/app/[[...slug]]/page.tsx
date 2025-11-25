import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { getSiteBySlug } from "@notepub/core";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { Sidebar } from "@/components/Sidebar";
import { getFolderBySlugPath, getIndexData, getNoteBySlug } from "@/lib/notes";
import { FormHandler } from "@/components/FormHandler";
import styles from "./page.module.css";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

type PageParams = {
  slug?: string[];
};

export async function generateMetadata({ params }: { params: { slug?: string[] } }): Promise<Metadata> {
  const hostHeader = headers().get("host") || "";
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(hostHeader);
  if (!siteSlug) return {};
  const site = await getSiteBySlug(siteSlug);
  if (!site) return {};
  const slugSegments = Array.isArray(params.slug) ? params.slug.map(decodeURIComponent) : [];
  const slugPath = slugSegments.join("/");
  const note = await getNoteBySlug(slugPath || siteSlug, site.s3Prefix);

  const title = note?.title || site.title;
  const origin = `https://${site.slug}.notepub.site`;
  const url = `${origin}/${slugPath}`;
  const description = site.ogDescription || "Site powered by Notepub";
  const ogImageRaw = site.ogImageUrl || "/og-default.svg";
  const ogImage = ogImageRaw.startsWith("http")
    ? ogImageRaw
    : `${origin}${ogImageRaw.startsWith("/") ? ogImageRaw : `/${ogImageRaw}`}`;

  return {
    title,
    description,
    openGraph: {
      title,
      url,
      siteName: site.title,
      description,
      images: [{ url: ogImage, width: 1200, height: 630 }],
      type: "website",
    },
  };
}

export default async function NotePage({ params }: { params: { slug?: string[] } }) {
  const hostHeader = headers().get("host") || "";
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(hostHeader);
  if (!siteSlug) {
    console.error("Missing slug (header/host)", hostHeader);
    notFound();
  }

  const site = await getSiteBySlug(siteSlug);
  if (!site) {
    console.error("Site not found for slug", siteSlug);
    notFound();
  }

  const slugSegments = Array.isArray(params.slug) ? params.slug.map(decodeURIComponent) : [];
  const slugPath = slugSegments.join("/");

  const index = await getIndexData(site.s3Prefix);
  if (slugSegments.length === 0 && index.flat.length > 0) {
    const defaultSlug =
      index.flat.find((item) => item.relativeKey === "index.md")?.slug ||
      index.flat.find((item) => item.slug === "welcome")?.slug ||
      index.flat[0].slug;
    redirect(`/${defaultSlug}`);
  }

  const note = await getNoteBySlug(slugPath, site.s3Prefix);
  if (!note) {
    const folder = await getFolderBySlugPath(slugSegments, site.s3Prefix);
    if (folder) {
      redirect(`/folders/${folder.slugPath.join("/")}`);
    }
    notFound();
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const flatForClient = index.flat.map(({ key: _key, ...rest }) => rest);
  const crumbs = note.breadcrumbs && note.breadcrumbs.length > 0 ? note.breadcrumbs : [{ title: note.title, href: null }];
  return (
    <div className={styles.shell}>
      <Sidebar tree={index.tree} flat={flatForClient} activeSlug={note.slug} siteTitle={site.title} siteAvatarUrl={site.ogImageUrl} />
      <main className={styles.content}>
        <div className={styles.noteWrapper}>
          <Breadcrumbs crumbs={crumbs} />
          <FormHandler />
          <article dangerouslySetInnerHTML={{ __html: note.html }} />
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
