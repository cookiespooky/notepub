"use server";

import { findUserByEmail } from "@notepub/core";
import { consumeResetToken } from "@/lib/codes";

export type FormState = { error?: string; success?: string };

export async function resetPasswordAction(_: FormState, formData: FormData): Promise<FormState> {
  const email = (formData.get("email") || "").toString().trim().toLowerCase();
  const code = (formData.get("code") || "").toString().trim();
  const password = (formData.get("password") || "").toString();

  if (!email || !code || !password) {
    return { error: "Заполните все поля." };
  }
  if (password.length < 6) {
    return { error: "Пароль должен быть не короче 6 символов." };
  }

  const user = await findUserByEmail(email);
  if (!user) {
    return { error: "Неверный email или код." };
  }

  const ok = await consumeResetToken(user.id, code, password);
  if (!ok) {
    return { error: "Неверный или истекший код." };
  }

  return { success: "Пароль обновлен. Теперь можно войти." };
}
