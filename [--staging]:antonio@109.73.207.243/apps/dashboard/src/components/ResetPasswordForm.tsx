"use client";

import { useFormState } from "react-dom";
import { resetPasswordAction, type FormState } from "@/app/reset-password/actions";
import styles from "@/app/auth.module.css";

const initialState: FormState = { error: "", success: "" };

export function ResetPasswordForm({ token }: { token?: string }) {
  const [state, formAction] = useFormState(resetPasswordAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <input type="hidden" name="token" value={token || ""} />
      <label className={styles.field}>
        <span>Новый пароль</span>
        <input type="password" name="password" minLength={6} required />
      </label>
      <label className={styles.field}>
        <span>Повторите пароль</span>
        <input type="password" name="confirmPassword" minLength={6} required />
      </label>
      {state.error && <p className={styles.error}>{state.error}</p>}
      {state.success && <p className={styles.success}>{state.success}</p>}
      <button type="submit" className={styles.primary}>
        Сбросить пароль
      </button>
    </form>
  );
}
