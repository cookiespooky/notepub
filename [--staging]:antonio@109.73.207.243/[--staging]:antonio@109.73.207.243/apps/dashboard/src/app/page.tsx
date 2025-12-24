import Link from "next/link";
import styles from "./page.module.css";

export default function LandingPage() {
  return (
    <main className={styles.hero}>
      <div className={styles.content}>
        <p className={styles.label}>Notepub</p>
        <h1 className={styles.title}>Сайт из заметок</h1>
        <div className={styles.sub}>
          Бесплатно до 1000 страниц (примерно)
        </div>
        <div className={styles.sub}>
          Настройка за пару минут и простое управление контентом
        </div>
        <div className={styles.sub}>
          SEO и Opengraph уже настроены
        </div>
        <div className={styles.actions}>
          <Link className={styles.secondary} href="https://about.notepub.site" target="_blank" rel="noreferrer">
            Подробнее
          </Link>
          <Link className={styles.primary} href="/login">
            Войти
          </Link>
        </div>
      </div>
    </main>
  );
}
