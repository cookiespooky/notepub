import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { getSiteBySlug } from "@notepub/core";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { Sidebar } from "@/components/Sidebar";
import { getIndexData, getNoteBySlug } from "@/lib/notes";
import { FormHandler } from "@/components/FormHandler";
import { LinkPreviewProvider } from "@/components/LinkPreviewProvider";
import { buildSiteUrl } from "@/lib/domains";
import { getCurrentUser } from "@/lib/session";
import { Reveal } from "@/components/Reveal";
import { getThemeSettings, themeToCssVars } from "@/lib/theme";
import { YandexMetrika } from "@/components/YandexMetrika";
import Script from "next/script";
import type { Metadata } from "next";
import { slugifySegment } from "@/lib/slug";

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
  const currentUser = await getCurrentUser();
  const isOwner = currentUser?.id === site.ownerId;
  const slugSegments = Array.isArray(params.slug) ? params.slug.map(decodeURIComponent) : [];
  const slugPath = slugSegments.join("/");
  let note: Awaited<ReturnType<typeof getNoteBySlug>> | null = null;

  if (slugSegments.length === 0) {
    const indexData = await getIndexData(site.s3Prefix, { includeDrafts: isOwner });
    const home = pickHomeNote(indexData.flat);
    if (home) {
      note = await getNoteBySlug(home.slug, site.s3Prefix, { includeDrafts: isOwner });
    }
  }

  if (!note) {
    note = await getNoteBySlug(slugPath || siteSlug, site.s3Prefix, { includeDrafts: isOwner });
  }
  if (!note) {
    const fallbackTitle = slugSegments.length > 0 ? slugSegments[slugSegments.length - 1] : site.title;
    return {
      title: fallbackTitle,
      description: site.title,
    };
  }

  const title = note.title || site.title;
  const origin = hostHeader ? `${proto}://${hostHeader}` : buildSiteUrl(site.slug, proto);
  const url = `${origin}${slugSegments.length === 0 ? "/" : `/${slugPath}`}`;
  const firstParagraph = extractFirstParagraph(note.html);
  const noteImage = slugSegments.length === 0 ? null : extractFirstImageSrc(note.html);
  const siteImageRaw = (site.ogImageUrl || "").trim();
  const logoFallback = "/logo.png";
  const chosenImageRaw = noteImage || siteImageRaw || logoFallback;
  const ogImage = toAbsoluteUrl(chosenImageRaw, origin) || chosenImageRaw;

  const description =
    (slugSegments.length === 0 ? site.ogDescription : firstParagraph) ||
    firstParagraph ||
    site.ogDescription ||
    note.preview ||
    "Site powered by Notepub";

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
      type: slugSegments.length > 0 ? "article" : "website",
    },
    twitter: {
      card: "summary",
      title,
      description,
      images: [ogImage],
    },
    robots: note.isDraft ? { index: false, follow: true } : undefined,
  };
}

export default async function NotePage({ params }: { params: { slug?: string[] } }) {
  const hostHeader = headers().get("host") || "";
  const proto = headers().get("x-forwarded-proto") || (hostHeader.includes("localhost") ? "http" : "https");
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
  const theme = await getThemeSettings((site as any).s3Prefix);
  const themeVars = themeToCssVars(theme);
  const yandexMetrikaId = (site as any).yandexMetrikaId as string | null | undefined;
  const currentUser = await getCurrentUser();
  const isOwner = currentUser?.id === site.ownerId;

  const slugSegments = Array.isArray(params.slug) ? params.slug.map(decodeURIComponent) : [];
  const slugPath = slugSegments.join("/");

  const index = await getIndexData(site.s3Prefix, { includeDrafts: isOwner });
  const home = pickHomeNote(index.flat);
  if (home && slugSegments.join("/") === home.slug) {
    redirect("/");
  }
  const previewMap = Object.fromEntries(index.flat.map(({ slug, title, preview }) => [slug, { title, preview }]));
  if (slugSegments.length === 0 && index.flat.length > 0) {
    const home = pickHomeNote(index.flat);
    if (home) {
      const note = await getNoteBySlug(home.slug, site.s3Prefix, { includeDrafts: isOwner });
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
          <>
            <YandexMetrika counterId={yandexMetrikaId} />
            {themeVars && (
              <style
                dangerouslySetInnerHTML={{
                  __html: `:root{${themeVars}}`,
                }}
              />
            )}
            <div className={`page-shell ${hideSidebar ? "page-shell--no-sidebar" : ""}`}>
              {!hideSidebar && (
                <Sidebar
                  siteSlug={site.slug}
                  categories={index.categories}
                  flat={flatForClient}
                  activeSlug={note.slug}
                  activeCategorySlug={note.category ? slugifySegment(note.category) || undefined : undefined}
                  siteTitle={site.title}
                  siteAvatarUrl={site.ogImageUrl}
                />
              )}
              <main className="page-main">
                <div className="page-content">
                  <FormHandler />
                  <LinkPreviewProvider previews={previewMap}>
                    <article className="prose" dangerouslySetInnerHTML={{ __html: note.html }} />
                  </LinkPreviewProvider>
                </div>
              </main>
            </div>
          </>
        );
      }
    }
    const fallbackSlug = index.flat[0].slug;
    redirect(`/${fallbackSlug}`);
  }

  const note = await getNoteBySlug(slugPath, site.s3Prefix, { includeDrafts: isOwner });
  if (!note) {
    notFound();
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const flatForClient = index.flat.map(({ key: _key, ...rest }) => rest);
  const crumbs = note.breadcrumbs && note.breadcrumbs.length > 0 ? note.breadcrumbs : [{ title: note.title, href: null }];
  const origin = hostHeader ? `${proto}://${hostHeader}` : buildSiteUrl(site.slug, proto);
  const canonical = `${origin}${slugSegments.length === 0 ? "/" : `/${slugPath}`}`;
  const breadcrumbJsonLd = buildBreadcrumbJsonLd(crumbs, canonical);
  const chosenImage = selectPreferredImage(note.html, site.ogImageUrl, origin, slugSegments.length > 0);
  const articleJsonLd =
    slugSegments.length > 0
      ? buildArticleJsonLd({
          title: note.title || site.title,
          description: note.preview || site.ogDescription || "",
          url: canonical,
          image: chosenImage,
          published: note.created,
          modified: note.updated,
          siteTitle: site.title,
        })
      : "";
  return (
    <>
      <YandexMetrika counterId={yandexMetrikaId} />
      {themeVars && (
        <style
          dangerouslySetInnerHTML={{
            __html: `:root{${themeVars}}`,
          }}
        />
      )}
      {breadcrumbJsonLd && (
        <Script id="breadcrumb-jsonld" type="application/ld+json" strategy="afterInteractive">
          {breadcrumbJsonLd}
        </Script>
      )}
      {articleJsonLd && (
        <Script id="article-jsonld" type="application/ld+json" strategy="afterInteractive">
          {articleJsonLd}
        </Script>
      )}
      <div className="page-shell">
        <Sidebar
          siteSlug={site.slug}
          categories={index.categories}
          flat={flatForClient}
          activeSlug={note.slug}
          activeCategorySlug={note.category ? slugifySegment(note.category) || undefined : undefined}
          siteTitle={site.title}
          siteAvatarUrl={site.ogImageUrl}
        />
        <main className="page-main">
          <Reveal>
            <div className="page-content">
              {note.isDraft && isOwner && <div className="draft-banner">Черновик. Видно только вам.</div>}
              {slugSegments.length > 0 && <Breadcrumbs crumbs={crumbs} />}
              <FormHandler />
              <LinkPreviewProvider previews={previewMap}>
                <article className="prose" dangerouslySetInnerHTML={{ __html: note.html }} />
              </LinkPreviewProvider>
            </div>
          </Reveal>
        </main>
      </div>
    </>
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

function pickHomeNote(flat: { relativeKey: string; slug: string; isHome?: boolean }[]) {
  const rootNotes = flat.filter((note) => note.relativeKey.split("/").filter(Boolean).length === 1);
  const homeCandidate = rootNotes.find((note) => note.isHome);
  if (homeCandidate) return homeCandidate;
  return rootNotes[0] || null;
}

function buildBreadcrumbJsonLd(crumbs: { title: string; href: string | null }[], canonical: string) {
  if (!crumbs || crumbs.length === 0) return "";
  const items = crumbs.map((crumb, idx) => {
    const itemUrl = crumb.href ? normalizeUrl(crumb.href, canonical) : canonical;
    return {
      "@type": "ListItem",
      position: idx + 1,
      name: crumb.title || canonical,
      item: itemUrl,
    };
  });
  return JSON.stringify({
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items,
  });
}

function buildArticleJsonLd(input: {
  title: string;
  description: string;
  url: string;
  image?: string | null;
  published?: string | null;
  modified?: string | null;
  siteTitle?: string | null;
}) {
  const data: Record<string, unknown> = {
    "@context": "https://schema.org",
    "@type": "Article",
    headline: input.title,
    description: input.description,
    url: input.url,
  };
  if (input.image) data.image = [input.image];
  if (input.published) data.datePublished = input.published;
  if (input.modified) data.dateModified = input.modified;
  if (input.siteTitle) {
    data.publisher = { "@type": "Organization", name: input.siteTitle };
  }
  return JSON.stringify(data);
}

function selectPreferredImage(noteHtml: string, siteImage: string | null | undefined, origin: string, hasSlug: boolean) {
  const noteImage = hasSlug ? extractFirstImageSrc(noteHtml) : "";
  const logoFallback = "/logo.png";
  const raw = noteImage || (siteImage || "").trim() || logoFallback;
  const absolute = toAbsoluteUrl(raw, origin);
  return absolute || raw;
}

function normalizeUrl(href: string, canonicalBase: string) {
  if (!href) return canonicalBase;
  if (/^https?:\/\//i.test(href)) return href;
  try {
    const url = new URL(href, canonicalBase);
    return url.toString();
  } catch {
    return canonicalBase;
  }
}

function extractFirstParagraph(html: string) {
  const paragraphRegex = /<p[^>]*>([\s\S]*?)<\/p>/gi;
  for (const match of html.matchAll(paragraphRegex)) {
    const inner = match[1] || "";
    const withoutTags = inner.replace(/<[^>]+>/g, " ");
    const decoded = decodeEntities(withoutTags).replace(/\s+/g, " ").trim();
    if (decoded) return decoded;
  }
  return "";
}

function extractFirstImageSrc(html: string) {
  const imageMatch = html.match(/<img[^>]+src=["']([^"']+)["'][^>]*>/i);
  return imageMatch?.[1]?.trim() || "";
}

function decodeEntities(input: string) {
  return input
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/g, "'")
    .replace(/&#x2F;/gi, "/");
}

function toAbsoluteUrl(raw: string, origin: string) {
  if (!raw) return null;
  if (/^https?:\/\//i.test(raw)) return raw;
  if (raw.startsWith("//")) {
    const protocol = origin.startsWith("https") ? "https:" : "http:";
    return `${protocol}${raw}`;
  }
  try {
    return new URL(raw, origin).toString();
  } catch {
    return null;
  }
}
