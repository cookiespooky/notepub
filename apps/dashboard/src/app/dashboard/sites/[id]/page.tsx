import Link from "next/link";
import { notFound } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { prisma } from "@notepub/db";
import { listObjects } from "@notepub/storage";
import { EditSiteForm } from "@/components/EditSiteForm";
import { DeleteSiteForm } from "@/components/DeleteSiteForm";
import styles from "../sites.module.css";

export default async function SiteDetailPage({ params }: { params: { id: string } }) {
  const user = await getCurrentUser();
  if (!user) return null;

  const site = await prisma.site.findUnique({ where: { id: params.id } });
  if (!site || site.ownerId !== user.id) {
    notFound();
  }

  const usage = await computeUsage(site.s3Prefix);
  const limitBytes = 50 * 1024 * 1024;
  const remaining = Math.max(limitBytes - usage, 0);

  return (
    <div className={styles.shell}>
      <div className={styles.header}>
        <div>
          <h1>{site.title}</h1>
          <p>Slug: {site.slug}.notepub.site</p>
        </div>
        <Link className={styles.primary} href="/dashboard/sites">
          Назад
        </Link>
      </div>

      <div className={styles.card}>
        <h3>Редактировать</h3>
        <EditSiteForm
          siteId={site.id}
          initialSlug={site.slug}
          initialTitle={site.title}
          initialOgImageUrl={site.ogImageUrl}
          initialOgDescription={site.ogDescription}
        />
      </div>

      <div className={styles.card}>
        <h3>Подключение Obsidian (Remotely Save)</h3>
        <p>Используйте эти значения для S3:</p>
        <ul>
          <li>S3 Endpoint: {process.env.S3_ENDPOINT}</li>
          <li>Bucket: {process.env.S3_BUCKET}</li>
          <li>Root folder: {site.s3Prefix}</li>
        </ul>
      </div>

      <div className={styles.card}>
        <h3>Ссылки</h3>
        <ul>
          <li>
            Публичный сайт:{" "}
            <Link href={`https://${site.slug}.notepub.site`} target="_blank">
              https://{site.slug}.notepub.site
            </Link>
          </li>
          <li>
            Карта файлов:{" "}
            <Link href={`https://${site.slug}.notepub.site/__map`} target="_blank">
              https://{site.slug}.notepub.site/__map
            </Link>
          </li>
        </ul>
      </div>

      <div className={styles.card}>
        <h3>Хранилище</h3>
        <p>
          Использовано: {(usage / (1024 * 1024)).toFixed(2)} МБ из 50 МБ. Осталось: {(remaining / (1024 * 1024)).toFixed(2)} МБ.
        </p>
      </div>

      <div className={styles.card}>
        <h3>Удалить сайт</h3>
        <DeleteSiteForm siteId={site.id} />
      </div>
    </div>
  );
}

async function computeUsage(prefix: string) {
  const objects = await listObjects(prefix);
  return objects.reduce((sum, obj) => sum + (obj.size || 0), 0);
}
