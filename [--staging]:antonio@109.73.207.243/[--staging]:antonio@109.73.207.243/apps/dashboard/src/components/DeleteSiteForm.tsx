"use client";

import { useState } from "react";
import { useFormState } from "react-dom";
import { deleteSiteAction } from "@/app/dashboard/sites/[id]/actions";
import styles from "@/app/dashboard/sites/sites.module.css";

const initialState = { error: "" };

export function DeleteSiteForm({ siteId }: { siteId: string }) {
  const [state, formAction] = useFormState(deleteSiteAction, initialState);
  const [open, setOpen] = useState(false);

  return (
    <>
      <button type="button" className={styles.danger} onClick={() => setOpen(true)}>
        Удалить сайт
      </button>

      {open && (
        <div className={styles.modalOverlay} role="dialog" aria-modal="true">
          <div className={styles.modal}>
            <h3>Удалить сайт?</h3>
            <p>Это действие удалит настройки и доступ к сайту. Вы уверены, что хотите продолжить?</p>
            {state?.error && <p className={styles.error}>{state.error}</p>}
            <form action={formAction} className={styles.modalActions}>
              <input type="hidden" name="id" value={siteId} />
              <button type="button" className={styles.modalSecondary} onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button type="submit" className={styles.modalPrimary}>
                Удалить
              </button>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
