"use client";

import { useFormState } from "react-dom";
import { resetPasswordAction, type FormState } from "@/app/reset-password/actions";
import styles from "@/app/auth.module.css";

const initialState: FormState = { error: "", success: "" };

export function ResetPasswordForm() {
  const [state, formAction] = useFormState(resetPasswordAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.field}>
        <span>Email</span>
        <input type="email" name="email" required />
      </label>
      <label className={styles.field}>
        <span>Код</span>
        <input name="code" inputMode="numeric" pattern="[0-9]*" minLength={4} maxLength={6} required />
      </label>
      <label className={styles.field}>
        <span>Новый пароль</span>
        <input type="password" name="password" minLength={6} required />
      </label>
      {state.error && <p className={styles.error}>{state.error}</p>}
      {state.success && <p className={styles.success}>{state.success}</p>}
      <button type="submit" className={styles.primary}>
        Сбросить пароль
      </button>
    </form>
  );
}
