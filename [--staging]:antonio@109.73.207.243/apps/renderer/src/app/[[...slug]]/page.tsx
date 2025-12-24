import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { getSiteBySlug } from "@notepub/core";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { Sidebar } from "@/components/Sidebar";
import { getFolderBySlugPath, getIndexData, getNoteBySlug } from "@/lib/notes";
import { FormHandler } from "@/components/FormHandler";
import { LinkPreviewProvider } from "@/components/LinkPreviewProvider";
import { buildSiteUrl, getAppUrl } from "@/lib/domains";
import styles from "./page.module.css";
import type { Metadata } from "next";

export const dynamic = "force-dynamic";

type PageParams = {
  slug?: string[];
};

export async function generateMetadata({ params }: { params: { slug?: string[] } }): Promise<Metadata> {
  const hostHeader = headers().get("host") || "";
  const proto = headers().get("x-forwarded-proto") || (hostHeader.includes("localhost") ? "http" : "https");
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(hostHeader);
  if (!siteSlug) return {};
  const site = await getSiteBySlug(siteSlug);
  if (!site) return {};
  const slugSegments = Array.isArray(params.slug) ? params.slug.map(decodeURIComponent) : [];
  const slugPath = slugSegments.join("/");
  let note: Awaited<ReturnType<typeof getNoteBySlug>> | null = null;

  if (slugSegments.length === 0) {
    const indexData = await getIndexData(site.s3Prefix);
    const home = pickHomeNote(indexData.flat);
    if (home) {
      note = await getNoteBySlug(home.slug, site.s3Prefix);
    }
  }

  if (!note) {
    note = await getNoteBySlug(slugPath || siteSlug, site.s3Prefix);
  }
  if (!note) return {};

  const title = note.title || site.title;
  const origin = hostHeader ? `${proto}://${hostHeader}` : buildSiteUrl(site.slug, proto);
  const url = `${origin}${slugSegments.length === 0 ? "/" : `/${slugPath}`}`;
  const description = site.ogDescription || "Site powered by Notepub";
  const ogImageRaw = site.ogImageUrl || `${getAppUrl()}/og-default.svg`;
  const ogImage = ogImageRaw.startsWith("http")
    ? ogImageRaw
    : `${origin}${ogImageRaw.startsWith("/") ? ogImageRaw : `/${ogImageRaw}`}`;

  return {
    title,
    description,
    alternates: { canonical: url },
    openGraph: {
      title,
      url,
      siteName: site.title,
      description,
      images: [{ url: ogImage, width: 1200, height: 630 }],
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [ogImage],
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
  const home = pickHomeNote(index.flat);
  if (home && slugSegments.join("/") === home.slug) {
    redirect("/");
  }
  const previewMap = Object.fromEntries(index.flat.map(({ slug, title, preview }) => [slug, { title, preview }]));
  if (slugSegments.length === 0 && index.flat.length > 0) {
    const home = pickHomeNote(index.flat);
    if (home) {
      const note = await getNoteBySlug(home.slug, site.s3Prefix);
      if (note) {
        const flatForClient = index.flat.map(({ key: _key, ...rest }) => rest);
        const hideSidebar = Boolean((site as any).hideSidebarOnHome) && home.slug === note.slug;
        const crumbs =
          note.breadcrumbs && note.breadcrumbs.length > 0
            ? note.breadcrumbs.map((crumb, idx, arr) =>
                idx === arr.length - 1 ? { ...crumb, href: null } : crumb,
              )
            : [{ title: note.title, href: null }];

        return (
          <div className={`${styles.shell} ${hideSidebar ? styles.noSidebar : ""}`}>
            {!hideSidebar && (
              <Sidebar
                tree={index.tree}
                flat={flatForClient}
                activeSlug={note.slug}
                siteTitle={site.title}
                siteAvatarUrl={site.ogImageUrl}
              />
            )}
            <main className={styles.content}>
              <div className={styles.noteWrapper}>
                <FormHandler />
                <LinkPreviewProvider previews={previewMap}>
                  <article dangerouslySetInnerHTML={{ __html: note.html }} />
                </LinkPreviewProvider>
              </div>
            </main>
          </div>
        );
      }
    }
    const fallbackSlug = index.flat[0].slug;
    redirect(`/${fallbackSlug}`);
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
          {slugSegments.length > 0 && <Breadcrumbs crumbs={crumbs} />}
          <FormHandler />
          <LinkPreviewProvider previews={previewMap}>
            <article dangerouslySetInnerHTML={{ __html: note.html }} />
          </LinkPreviewProvider>
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

function pickHomeNote(flat: { relativeKey: string; slug: string; isHome?: boolean; isFolderIndex?: boolean }[]) {
  const rootNotes = flat.filter((note) => !note.isFolderIndex && note.relativeKey.split("/").filter(Boolean).length === 1);
  const homeCandidate = rootNotes.find((note) => note.isHome);
  if (homeCandidate) return homeCandidate;
  return rootNotes[0] || null;
}
