import Link from "next/link";
import styles from "./page.module.css"; //

export default function LandingPage() {
  return (
    <main className={styles.hero}>
      <div className={styles.content}>
        <p className={styles.label}>Notepub</p>
        <h1 className={styles.title}>Сделай сайт из Obsidian за 5 минут</h1>
        <p className={styles.sub}>
          Загрузи vault в S3, укажи slug — получи slug.notepub.site с готовым рендером.
        </p>
        <div className={styles.actions}>
          <Link className={styles.primary} href="/signup">
            Start for free
          </Link>
          <Link className={styles.secondary} href="/dashboard">
            Open dashboard
          </Link>
        </div>
      </div>
    </main>
  );
}
