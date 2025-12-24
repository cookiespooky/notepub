import { headers } from "next/headers";
import { notFound } from "next/navigation";
import Link from "next/link";
import { Sidebar } from "@/components/Sidebar";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { getSiteBySlug } from "@notepub/core";
import { getFolderBySlugPath, getIndexData } from "@/lib/notes";
import styles from "./page.module.css";

export const dynamic = "force-dynamic";

export default async function FolderPage({ params }: { params: Promise<{ slug: string[] }> }) {
  const hostHeader = headers().get("host") || "";
  const siteSlug = extractSlugFromHost(hostHeader);
  const site = siteSlug ? await getSiteBySlug(siteSlug) : null;
  if (!site) notFound();

  const resolved = await params;
  const slugPath = Array.isArray(resolved.slug) ? resolved.slug.map(decodeURIComponent) : [resolved.slug];
  const [index, folder] = await Promise.all([getIndexData(site.s3Prefix), getFolderBySlugPath(slugPath, site.s3Prefix)]);
  if (!folder) notFound();

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const flatForClient = index.flat.map(({ key: _key, ...rest }) => rest);

  return (
    <div className={styles.shell}>
      <Sidebar tree={index.tree} flat={flatForClient} activeSlug="" siteTitle={site.title} siteAvatarUrl={site.ogImageUrl} />
      <main className={styles.content}>
        <div className={styles.wrapper}>
          <Breadcrumbs crumbs={folder.breadcrumbs} />
          <h1>{folder.title}</h1>

          {folder.folders.length > 0 && <div className={styles.sectionTitle}>Папки</div>}
          <div className={styles.list}>
            {folder.folders.map((child) => (
              <Link key={child.slugPath.join("/")} href={`/folders/${child.slugPath.join("/")}`} className={styles.item}>
                {child.title}
              </Link>
            ))}
          </div>

          {folder.notes.length > 0 && <div className={styles.sectionTitle}></div>}
          <div className={styles.list}>
            {folder.notes.map((note) => (
              <Link key={note.slug} href={`/${note.slug}`} className={styles.item}>
                {note.title}
              </Link>
            ))}
          </div>
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
