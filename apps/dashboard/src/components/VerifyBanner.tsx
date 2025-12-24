"use client";

import { useTransition, useState } from "react";
import { resendVerificationAction } from "@/app/verify-email/actions";
import styles from "@/app/dashboard/layout.module.css";

export function VerifyBanner({ email, verified }: { email: string; verified: boolean }) {
  const [pending, startTransition] = useTransition();
  const [status, setStatus] = useState<"idle" | "sent" | "error">("idle");
  const [message, setMessage] = useState<string>("");

  if (verified) return null;

  const onSend = () => {
    startTransition(() => {
      void (async () => {
        const res = await resendVerificationAction();
        if (res.error) {
          setStatus("error");
          setMessage(res.error);
        } else {
          setStatus("sent");
          setMessage(res.success || "");
        }
      })();
    });
  };

  return (
    <div className={styles.banner}>
      <span>
        Подтвердите email {email}. Введите код из письма или запросите новый.
      </span>
      <button className={styles.bannerButton} onClick={onSend} disabled={pending}>
        {pending ? "Отправляем..." : status === "sent" ? "Отправлено" : "Отправить код"}
      </button>
      {message && <span>{message}</span>}
    </div>
  );
}
