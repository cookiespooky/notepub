"use server";

import { issueVerificationCode, consumeVerificationCode } from "@/lib/codes";
import { sendVerificationEmail } from "@/lib/emails";
import { getCurrentUser, setSessionVerified } from "@/lib/session";

export type FormState = { error?: string; success?: string };

export async function resendVerificationAction(): Promise<FormState> {
  const user = await getCurrentUser();
  if (!user) {
    return { error: "Нужно войти, чтобы отправить код." };
  }
  if (user.emailVerified) {
    return { success: "Email уже подтвержден." };
  }
  try {
    const code = await issueVerificationCode(user.id);
    await sendVerificationEmail(user.email, code);
  } catch (error) {
    console.error("Failed to send verification email", error);
    return { error: "Не удалось отправить письмо. Проверьте SMTP настройки." };
  }
  return { success: "Код отправлен на почту." };
}

export async function verifyEmailAction(_: FormState, formData: FormData): Promise<FormState> {
  const code = (formData.get("code") || "").toString().trim();
  const user = await getCurrentUser();

  if (!user) {
    return { error: "Нужно войти, чтобы подтвердить email." };
  }
  if (user.emailVerified) {
    return { success: "Email уже подтвержден." };
  }
  if (!code) {
    return { error: "Введите код подтверждения." };
  }

  const ok = await consumeVerificationCode(user.id, code);
  if (!ok) {
    return { error: "Неверный или истекший код." };
  }
  setSessionVerified(true);
  return { success: "Email подтвержден." };
}

export async function verifyEmailByToken(token: string): Promise<FormState> {
  const user = await getCurrentUser();
  if (!user) return { error: "Нужно войти, чтобы подтвердить email." };
  if (user.emailVerified) return { success: "Email уже подтвержден." };
  if (!token) return { error: "Некорректная ссылка подтверждения." };
  const ok = await consumeVerificationCode(user.id, token.trim());
  if (!ok) return { error: "Неверная или истекшая ссылка." };
  setSessionVerified(true);
  return { success: "Email подтвержден." };
}
