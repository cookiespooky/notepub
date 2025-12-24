import { loadEnv } from "@notepub/env";
import { sendMail } from "./mailer";

const env = loadEnv();
const appUrl = env.APP_URL || "https://notepub.site";

export async function sendVerificationEmail(to: string, token: string) {
  const subject = "Notepub: подтвердите email";
  const link = `${appUrl.replace(/\/+$/, "")}/verify-email/confirm?token=${encodeURIComponent(token)}`;
  const text = `Подтвердите email для Notepub:

Перейдите по ссылке:
${link}

Ссылка действует 15 минут. Если вы не запрашивали подтверждение, игнорируйте это письмо.`;
  await sendMail(to, subject, text);
}

export async function sendPasswordResetEmail(to: string, code: string) {
  const subject = "Notepub: сброс пароля";
  const link = `${appUrl.replace(/\/+$/, "")}/reset-password?token=${encodeURIComponent(code)}`;
  const text = `Сброс пароля в Notepub

Перейдите по ссылке, чтобы задать новый пароль (ссылка действительна 15 минут):
${link}

Если вы не запрашивали сброс, просто игнорируйте это письмо.`;
  await sendMail(to, subject, text);
}
