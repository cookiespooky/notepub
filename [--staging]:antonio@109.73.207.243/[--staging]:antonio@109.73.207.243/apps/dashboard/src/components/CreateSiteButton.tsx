"use client";

import Link from "next/link";
import styles from "@/app/dashboard/sites/sites.module.css";

export function CreateSiteButton({ canCreate }: { canCreate: boolean }) {
  if (canCreate) {
    return (
      <Link className={styles.primary} href="/dashboard/sites/new">
        Новый сайт
      </Link>
    );
  }

  const onClick = () => {
    alert("Лимит: 1 сайт на пользователя. Скоро добавим платные планы.");
  };

  return (
    <button type="button" className={styles.primary} onClick={onClick} aria-label="Лимит сайтов">
      Новый сайт
    </button>
  );
}
