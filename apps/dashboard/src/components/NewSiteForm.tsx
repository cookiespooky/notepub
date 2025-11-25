"use client";

import { useFormState } from "react-dom";
import { createSiteAction } from "../app/dashboard/sites/new/actions";
import styles from "../app/dashboard/sites/sites.module.css";

const initialState = { error: "" };

export function NewSiteForm() {
  const [state, formAction] = useFormState(createSiteAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.field}>
        <span>Slug</span>
        <input
          name="slug"
          placeholder="myblog"
          minLength={1}
          maxLength={63}
          required
        />
        <small>Используется в домене: slug.notepub.site</small>
      </label>
      <label className={styles.field}>
        <span>Название</span>
        <input name="title" placeholder="Мой сайт" required />
      </label>
      {state?.error && <p className={styles.error}>{state.error}</p>}
      <button type="submit" className={styles.primary}>
        Создать
      </button>
    </form>
  );
}
