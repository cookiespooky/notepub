import Link from "next/link";
import { notFound } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { prisma } from "@notepub/db";
import { EditSiteForm } from "@/components/EditSiteForm";
import { DeleteSiteForm } from "@/components/DeleteSiteForm";
import { SiteSyncBlock } from "@/components/SiteSyncBlock";
import { buildSiteUrl } from "@/lib/domains";
import { ButtonLink } from "@/components/ButtonLink";
import styles from "../sites.module.css";

export default async function SiteDetailPage({ params }: { params: { id: string } }) {
  const user = await getCurrentUser();
  if (!user) return null;

  const site = await prisma.site.findUnique({
    where: { id: params.id },
    select: {
      id: true,
      slug: true,
      title: true,
      ogImageUrl: true,
      ogDescription: true,
      s3Prefix: true,
      vaultQuotaBytes: true,
      vaultUsedBytes: true,
      ownerId: true,
      hideSidebarOnHome: true,
      yandexMetrikaId: true,
      createdAt: true,
      updatedAt: true,
    },
  });
  if (!site || site.ownerId !== user.id) {
    notFound();
  }

  const usage = Number(site.vaultUsedBytes || 0);
  const limitBytes = Number(site.vaultQuotaBytes || 10 * 1024 * 1024);
  const remaining = Math.max(limitBytes - usage, 0);

  return (
    <div className={styles.shell}>
      <div className={styles.header}>
        <div style={{flex: 1}}>
          <h1>{site.title}</h1>
          <Link href={buildSiteUrl(site.slug)} target="_blank">{buildSiteUrl(site.slug)}</Link>
        </div>
        <div className={styles.actions}>
          <ButtonLink variant="secondary" href="/dashboard/sites">
            Назад
          </ButtonLink>
          {/*<ButtonLink variant="primary" href={buildSiteUrl(site.slug) + "/editor"} target="_blank">
            Открыть редактор
          </ButtonLink>*/}
        </div>
      </div>
      <div className={styles.card}>
        <h3>Редактирование сайта</h3>
        <EditSiteForm
          siteId={site.id}
          initialSlug={site.slug}
          initialTitle={site.title}
          initialOgImageUrl={site.ogImageUrl}
          initialOgDescription={site.ogDescription}
          initialHideSidebarOnHome={site.hideSidebarOnHome}
          initialYandexMetrikaId={site.yandexMetrikaId}
        />
      </div>

      <SiteSyncBlock siteId={site.id} siteSlug={site.slug} />

      <div className={styles.card}>
        <h3>Ссылки</h3>
        <ul>
          <li>
            Публичный сайт:{" "}
            <Link href={buildSiteUrl(site.slug)} target="_blank">
              {buildSiteUrl(site.slug)}
            </Link>
          </li>
          <li>
            Карта сайта:{" "}
            <Link href={`${buildSiteUrl(site.slug)}/sitemap.xml`} target="_blank">
              {buildSiteUrl(site.slug)}/sitemap.xml
            </Link>
          </li>
          <li>
            Robots:{" "}
            <Link href={`${buildSiteUrl(site.slug)}/robots.txt`} target="_blank">
              {buildSiteUrl(site.slug)}/robots.txt
            </Link>
          </li>
        </ul>
      </div>

      <div className={styles.card}>
        <h3>Хранилище</h3>
        <p>Использовано: {(usage / (1024 * 1024)).toFixed(2)} МБ из {(limitBytes / (1024 * 1024)).toFixed(2)} МБ. Осталось: {(remaining / (1024 * 1024)).toFixed(2)} МБ.</p>
      </div>

      <div className={styles.card}>
        <h3>Danger zone</h3>
        <DeleteSiteForm siteId={site.id} />
      </div>
    </div>
  );
}
