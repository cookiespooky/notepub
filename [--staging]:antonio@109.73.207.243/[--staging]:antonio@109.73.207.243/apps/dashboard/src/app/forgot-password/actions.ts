"use server";

import { findUserByEmail } from "@notepub/core";
import { issueResetToken } from "@/lib/codes";
import { sendPasswordResetEmail } from "@/lib/emails";

export type FormState = { error?: string; success?: string };

export async function forgotPasswordAction(_: FormState, formData: FormData): Promise<FormState> {
  const email = (formData.get("email") || "").toString().trim().toLowerCase();
  if (!email) {
    return { error: "Укажите email." };
  }

  const user = await findUserByEmail(email);
  if (user) {
    try {
      const token = await issueResetToken(user.id);
      await sendPasswordResetEmail(email, token);
    } catch (error) {
      console.error("Failed to send reset email", error);
      return { error: "Не удалось отправить письмо. Проверьте SMTP настройки." };
    }
  }

  return { success: "Если такой аккаунт есть, мы отправили код на почту." };
}
