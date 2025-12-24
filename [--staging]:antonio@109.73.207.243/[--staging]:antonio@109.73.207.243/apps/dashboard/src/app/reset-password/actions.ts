"use server";

import { consumeResetToken } from "@/lib/codes";

export type FormState = { error?: string; success?: string };

export async function resetPasswordAction(_: FormState, formData: FormData): Promise<FormState> {
  const token = (formData.get("token") || "").toString().trim();
  const password = (formData.get("password") || "").toString();
  const confirm = (formData.get("confirmPassword") || "").toString();

  if (!token) {
    return { error: "Некорректная ссылка для сброса. Запросите новый сброс пароля." };
  }
  if (!password || !confirm) {
    return { error: "Введите новый пароль." };
  }
  if (password.length < 6) {
    return { error: "Пароль должен быть не короче 6 символов." };
  }
  if (password !== confirm) {
    return { error: "Пароли не совпадают." };
  }

  const ok = await consumeResetToken(token, password);
  if (!ok) {
    return { error: "Неверный или истекший токен." };
  }

  return { success: "Пароль обновлен. Теперь можно войти." };
}
