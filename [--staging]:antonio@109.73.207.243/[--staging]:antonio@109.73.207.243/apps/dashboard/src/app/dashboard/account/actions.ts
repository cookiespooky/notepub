"use server";

import { findUserWithPasswordByEmail } from "@notepub/core";
import { prisma } from "@notepub/db";
import { isUniqueConstraintError, setUserPassword, verifyPassword } from "@/lib/auth";
import { issueVerificationCode } from "@/lib/codes";
import { sendVerificationEmail } from "@/lib/emails";
import { getCurrentUser, setSession } from "@/lib/session";

export type FormState = { error?: string; success?: string };

export async function updateEmailAction(_: FormState, formData: FormData): Promise<FormState> {
  const user = await getCurrentUser();
  if (!user) return { error: "Нужно войти." };

  const email = (formData.get("email") || "").toString().trim().toLowerCase();
  const currentPassword = (formData.get("currentPassword") || "").toString();
  if (!email) return { error: "Введите email." };
  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) return { error: "Некорректный email." };
  if (email === user.email) return { success: "Email уже обновлен." };
  if (!currentPassword) return { error: "Введите текущий пароль." };

  const userWithPassword = await findUserWithPasswordByEmail(user.email);
  const existingHash = userWithPassword?.password?.hash;
  if (existingHash) {
    const ok = await verifyPassword(currentPassword, existingHash);
    if (!ok) return { error: "Текущий пароль неверный." };
  }

  try {
    await prisma.user.update({
      where: { id: user.id },
      data: { email, emailVerified: false },
    });
  } catch (error) {
    if (isUniqueConstraintError(error, "email")) {
      return { error: "Этот email уже используется." };
    }
    console.error("Failed to update email", error);
    return { error: "Не удалось обновить email." };
  }

  setSession(user.id, false);

  try {
    const code = await issueVerificationCode(user.id);
    await sendVerificationEmail(email, code);
  } catch (error) {
    console.error("Failed to send verification email after email change", error);
  }

  return { success: "Email обновлен. Мы отправили письмо для подтверждения." };
}

export async function updatePasswordAction(_: FormState, formData: FormData): Promise<FormState> {
  const user = await getCurrentUser();
  if (!user) return { error: "Нужно войти." };

  const currentPassword = (formData.get("currentPassword") || "").toString();
  const newPassword = (formData.get("newPassword") || "").toString();
  const confirmPassword = (formData.get("confirmPassword") || "").toString();

  if (!currentPassword || !newPassword || !confirmPassword) return { error: "Заполните все поля." };
  if (newPassword.length < 6) return { error: "Пароль должен быть не короче 6 символов." };
  if (newPassword !== confirmPassword) return { error: "Пароли не совпадают." };

  const userWithPassword = await findUserWithPasswordByEmail(user.email);
  const existingHash = userWithPassword?.password?.hash;
  if (existingHash) {
    const ok = await verifyPassword(currentPassword, existingHash);
    if (!ok) return { error: "Текущий пароль неверный." };
  }

  try {
    await setUserPassword(user.id, newPassword);
  } catch (error) {
    console.error("Failed to update password", error);
    return { error: "Не удалось обновить пароль." };
  }

  return { success: "Пароль обновлен." };
}
