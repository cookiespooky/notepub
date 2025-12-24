"use client";

import { useFormState } from "react-dom";
import { signupAction } from "../app/signup/actions";
import styles from "../app/auth.module.css";

const initialState = { error: "" };

export function SignupForm() {
  const [state, formAction] = useFormState(signupAction, initialState);

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.field}>
        <span>Email</span>
        <input type="email" name="email" required autoComplete="email" />
      </label>
      <label className={styles.field}>
        <span>Пароль</span>
        <input type="password" name="password" minLength={6} required autoComplete="new-password" />
      </label>
      {state?.error && <p className={styles.error}>{state.error}</p>}
      <button type="submit" className={styles.primary}>
        Sign up
      </button>
    </form>
  );
}
