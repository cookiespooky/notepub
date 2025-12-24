"use client";

import { useEffect, useMemo, useState, useTransition } from "react";
import Link from "next/link";
import { useFormState } from "react-dom";
import { loginAction } from "@/app/login/actions";
import { signupAction } from "@/app/signup/actions";
import styles from "@/app/auth.module.css";

type Mode = "email" | "login" | "signup";

const initialState = { error: "" };

function PasswordField({
  label,
  name,
  autoComplete,
}: {
  label: string;
  name: string;
  autoComplete: string;
}) {
  const [show, setShow] = useState(false);
  return (
    <label className={styles.field}>
      <span>{label}</span>
      <div className={styles.passwordRow}>
        <input type={show ? "text" : "password"} name={name} minLength={6} required autoComplete={autoComplete} />
        <button type="button" className={styles.linkButton} onClick={() => setShow((v) => !v)}>
          {show ? "Скрыть" : "Показать"}
        </button>
      </div>
    </label>
  );
}

export function UnifiedAuthForm() {
  const [mode, setMode] = useState<Mode>("email");
  const [email, setEmail] = useState("");
  const [checking, startTransition] = useTransition();
  const [checkError, setCheckError] = useState("");

  const action = useMemo(() => (mode === "login" ? loginAction : signupAction), [mode]);
  const [state, formAction] = useFormState(action, initialState);

  useEffect(() => {
    if (state?.error) {
      // stay on same mode so user can correct errors
      return;
    }
  }, [state?.error]);

  const handleCheckEmail = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const rawEmail = (formData.get("email") || "").toString().trim().toLowerCase();
    setCheckError("");
    if (!rawEmail) {
      setCheckError("Введите email");
      return;
    }
    startTransition(() => {
      void (async () => {
        try {
          const res = await fetch("/api/auth/check-email", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ email: rawEmail }),
          });
          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error(data.error || "Не удалось проверить email");
          }
          const data = await res.json();
          setEmail(rawEmail);
          setMode(data.exists ? "login" : "signup");
        } catch (error) {
          setCheckError(error instanceof Error ? error.message : "Не удалось проверить email");
        }
      })();
    });
  };

  return (
    <>
      {mode === "email" ? (
        <form onSubmit={handleCheckEmail} className={styles.form}>
          <label className={styles.field}>
            <span>Email</span>
            <input
              type="email"
              name="email"
              required
              autoComplete="email"
              defaultValue={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>
          {checkError && <p className={styles.error}>{checkError}</p>}
          <button type="submit" className={styles.primary} disabled={checking}>
            {checking ? "Проверяем..." : "Продолжить"}
          </button>
        </form>
      ) : (
        <form action={formAction} className={styles.form}>
          <input type="hidden" name="email" value={email} />
          <div className={styles.fieldRow}>
            <div>
              <div className={styles.meta}>Email</div>
              <div className={styles.emailPill}>{email}</div>
            </div>
            <button type="button" className={styles.linkButton} onClick={() => setMode("email")}>
              Изменить email
            </button>
          </div>

          {mode === "signup" ? (
            <>
              <PasswordField label="Пароль" name="password" autoComplete="new-password" />
              <PasswordField label="Повторите пароль" name="passwordConfirm" autoComplete="new-password" />
            </>
          ) : (
            <PasswordField label="Пароль" name="password" autoComplete="current-password" />
          )}

          {state?.error && <p className={styles.error}>{state.error}</p>}

          <button type="submit" className={styles.primary}>
            {mode === "login" ? "Войти" : "Создать аккаунт"}
          </button>

          {mode === "login" && (
            <p className={styles.meta}>
              <Link href="/forgot-password">Забыли пароль?</Link>
            </p>
          )}
        </form>
      )}
    </>
  );
}
