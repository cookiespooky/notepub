import Link from "next/link";
import { listSitesForUser } from "@notepub/core";
import { getCurrentUser } from "@/lib/session";
import { CreateSiteButton } from "@/components/CreateSiteButton";
import styles from "./sites.module.css";

type SiteListItem = Awaited<ReturnType<typeof listSitesForUser>>[number];

export default async function SitesPage() {
  const user = await getCurrentUser();
  if (!user) {
    return null;
  }

  const sites = await listSitesForUser(user.id);

  return (
    <div className={styles.shell}>
      <div className={styles.header}>
        <div>
          <h1>Ваши сайты</h1>
          <p>Создавайте и подключайте Obsidian vault через S3</p>
        </div>
        <CreateSiteButton canCreate={sites.length === 0} />
      </div>

      {sites.length === 0 ? (
        <p className={styles.empty}>Пока нет сайтов. Создайте первый.</p>
      ) : (
        <div className={styles.list}>
          {sites.map((site: SiteListItem) => (
            <Link key={site.id} href={`/dashboard/sites/${site.id}`} className={styles.card}>
              <div className={styles.cardHeader}>
                <span className={styles.slug}>{site.slug}.notepub.site</span>
                <span className={styles.pill}>S3: {site.s3Prefix}</span>
              </div>
              <h3>{site.title}</h3>
              <p>Обновлено: {site.updatedAt.toDateString()}</p>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
