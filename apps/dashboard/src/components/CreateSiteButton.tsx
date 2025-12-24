"use client";

import Link from "next/link";
import { useState } from "react";
import styles from "@/app/dashboard/sites/sites.module.css";

export function CreateSiteButton({ canCreate }: { canCreate: boolean }) {
  const [showLimit, setShowLimit] = useState(false);

  if (canCreate) {
    return (
      <Link className={styles.primary} href="/dashboard/sites/new">
        Новый сайт
      </Link>
    );
  }

  return (
    <>
      <button type="button" className={styles.primary} onClick={() => setShowLimit(true)} aria-label="Лимит сайтов">
        Новый сайт
      </button>

      {showLimit && (
        <div className={styles.modalOverlay} role="dialog" aria-modal="true" onClick={() => setShowLimit(false)}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <h3>Лимит сайтов</h3>
            <p>Сейчас можно создать только 1 сайт. Скоро добавим больше.</p>
            <div className={styles.modalActions}>
              <button type="button" className={styles.modalPrimary} onClick={() => setShowLimit(false)}>
                Понятно
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
