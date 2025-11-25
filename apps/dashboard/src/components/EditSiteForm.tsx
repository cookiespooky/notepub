"use client";

import { useFormState } from "react-dom";
import { updateSiteAction } from "@/app/dashboard/sites/[id]/actions";
import styles from "@/app/dashboard/sites/sites.module.css";

const initialState = { error: "" };

export function EditSiteForm({
  siteId,
  initialSlug,
  initialTitle,
  initialOgImageUrl,
  initialOgDescription,
}: {
  siteId: string;
  initialSlug: string;
  initialTitle: string;
  initialOgImageUrl?: string | null;
  initialOgDescription?: string | null;
}) {
  const [state, formAction] = useFormState(updateSiteAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <input type="hidden" name="id" value={siteId} />
      <label className={styles.field}>
        <span>Slug</span>
        <input name="slug" defaultValue={initialSlug} required />
        <small>Используется в домене: slug.notepub.site</small>
      </label>
      <label className={styles.field}>
        <span>Название</span>
        <input name="title" defaultValue={initialTitle} required />
      </label>
      <label className={styles.field}>
        <span>OG Image URL (опционально)</span>
        <input name="ogImageUrl" defaultValue={initialOgImageUrl || ""} placeholder="https://... или путь в бакете" />
        <small>Если пусто — используется дефолтная картинка</small>
      </label>
      <label className={styles.field}>
        <span>OG Description (опционально)</span>
        <textarea name="ogDescription" defaultValue={initialOgDescription || ""} rows={3} />
      </label>
      {state?.error && <p className={styles.error}>{state.error}</p>}
      <button type="submit" className={styles.primary}>
        Сохранить
      </button>
    </form>
  );
}
