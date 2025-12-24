"use client";

import { useEffect, useState } from "react";
import { useFormState, useFormStatus } from "react-dom";
import { useRouter } from "next/navigation";
import { createSiteAction, type CreateSiteState } from "../app/dashboard/sites/new/actions";
import styles from "../app/dashboard/sites/sites.module.css";
import { getSitesBaseDomain } from "@/lib/domains";

const initialState: CreateSiteState = { error: "" };
const baseDomain = getSitesBaseDomain();

export function NewSiteForm() {
  const [state, formAction] = useFormState(createSiteAction, initialState);
  const router = useRouter();

  useEffect(() => {
    if (state?.success && state.redirectTo) {
      const timer = setTimeout(() => router.push(state.redirectTo as string), 600);
      return () => clearTimeout(timer);
    }
  }, [state?.success, state?.redirectTo, router]);

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.field}>
        <span>Выберите субдомен</span>
        <input
          name="slug"
          placeholder="myblog"
          minLength={1}
          maxLength={63}
          required
        />
        <small>Используется в домене: slug.{baseDomain}</small>
      </label>
      <label className={styles.field}>
        <span>Название</span>
        <input name="title" placeholder="Мой сайт" required />
      </label>
      <SubmitButton defaultLabel="Создать" state={state} />
    </form>
  );
}

function SubmitButton({ defaultLabel, state }: { defaultLabel: string; state: CreateSiteState }) {
  const { pending } = useFormStatus();
  const [label, setLabel] = useState(defaultLabel);

  useEffect(() => {
    if (pending) {
      setLabel("Сохранение");
      return;
    }
    if (state?.error) {
      setLabel("Ошибка");
    } else if (state?.success) {
      setLabel("Сохранено");
    } else {
      setLabel(defaultLabel);
    }
    const timer = setTimeout(() => setLabel(defaultLabel), 2000);
    return () => clearTimeout(timer);
  }, [pending, state?.error, state?.success, defaultLabel]);

  return (
    <div className={styles.actions}>
      <button type="submit" className={styles.primary} disabled={pending}>
        {label}
      </button>
      {state?.error && <p className={styles.error}>{state.error}</p>}
    </div>
  );
}
