import Link from "next/link";
import { NewSiteForm } from "@/components/NewSiteForm";
import styles from "../sites.module.css";

export default function NewSitePage() {
  return (
    <div className={styles.shell}>
      <div className={styles.header}>
        <div>
          <h1>Новый сайт</h1>
          <p>Укажите slug и название</p>
        </div>
        <Link className={styles.primary} href="/dashboard/sites">
          Назад
        </Link>
      </div>
      <NewSiteForm />
    </div>
  );
}
