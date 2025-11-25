"use client";

import { useFormState } from "react-dom";
import { deleteSiteAction } from "@/app/dashboard/sites/[id]/actions";
import styles from "@/app/dashboard/sites/sites.module.css";

const initialState = { error: "" };

export function DeleteSiteForm({ siteId }: { siteId: string }) {
  const [state, formAction] = useFormState(deleteSiteAction, initialState);
  return (
    <form action={formAction} className={styles.form}>
      <input type="hidden" name="id" value={siteId} />
      {state?.error && <p className={styles.error}>{state.error}</p>}
      <button type="submit" className={styles.danger}>
        Удалить сайт
      </button>
    </form>
  );
}
