"use client";

import Link from "next/link";
import { useFormState } from "react-dom";
import { loginAction } from "../app/login/actions";
import styles from "../app/auth.module.css";

const initialState = { error: "" };

export function LoginForm() {
  const [state, formAction] = useFormState(loginAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.field}>
        <span>Email</span>
        <input type="email" name="email" required autoComplete="email" />
      </label>
      <label className={styles.field}>
        <span>Пароль</span>
        <input type="password" name="password" minLength={6} required autoComplete="current-password" />
      </label>
      {state?.error && <p className={styles.error}>{state.error}</p>}
      <button type="submit" className={styles.primary}>
        Login
      </button>
      <p className={styles.meta}>
        <Link href="/forgot-password">Забыли пароль?</Link>
      </p>
    </form>
  );
}
