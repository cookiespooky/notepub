"use client";

import { useFormState } from "react-dom";
import { forgotPasswordAction, type FormState } from "@/app/forgot-password/actions";
import styles from "@/app/auth.module.css";

const initialState: FormState = { error: "", success: "" };

export function ForgotPasswordForm() {
  const [state, formAction] = useFormState(forgotPasswordAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.field}>
        <span>Email</span>
        <input type="email" name="email" required />
      </label>
      {state.error && <p className={styles.error}>{state.error}</p>}
      {state.success && <p className={styles.success}>{state.success}</p>}
      <button type="submit" className={styles.primary}>
        Отправить код
      </button>
    </form>
  );
}
