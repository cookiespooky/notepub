"use client";

import { useFormState } from "react-dom";
import { useState, useTransition } from "react";
import { verifyEmailAction, resendVerificationAction } from "@/app/verify-email/actions";
import styles from "@/app/auth.module.css";

type FormState = { error?: string; success?: string };
const initialState: FormState = { error: "", success: "" };

export function VerifyEmailForm({ email }: { email: string }) {
  const [state, formAction] = useFormState(verifyEmailAction, initialState);
  const [pending, startTransition] = useTransition();
  const [resendMessage, setResendMessage] = useState("");

  return (
    <div className={styles.form}>
      <label className={styles.field}>
        <span>Email</span>
        <input type="email" value={email} disabled />
      </label>

      <form action={formAction} className={styles.form}>
        <label className={styles.field}>
          <span>Код из письма</span>
          <input name="code" inputMode="numeric" pattern="[0-9]*" minLength={4} maxLength={6} required />
        </label>
        {state.error && <p className={styles.error}>{state.error}</p>}
        {state.success && <p className={styles.success}>{state.success}</p>}
        <button type="submit" className={styles.primary}>
          Подтвердить
        </button>
      </form>

      <button
        type="button"
        className={styles.secondary}
        onClick={() =>
          startTransition(async () => {
            const res = await resendVerificationAction();
            setResendMessage(res.success || res.error || "");
          })
        }
        disabled={pending}
      >
        {pending ? "Отправляем..." : "Отправить код повторно"}
      </button>
      {resendMessage && <p className={styles.meta}>{resendMessage}</p>}
    </div>
  );
}
