import { loadEnv } from "@notepub/env";
import { sendMail } from "./mailer";

const env = loadEnv();
const appUrl = env.APP_URL || "https://notepub.site";

export async function sendVerificationEmail(to: string, token: string) {
  const subject = "Notepub: подтвердите email";
  const link = `${appUrl.replace(/\/+$/, "")}/verify-email?token=${encodeURIComponent(token)}`;
  const text = `Подтвердите email для Notepub:

Перейдите по ссылке:
${link}

Ссылка действует 15 минут. Если вы не запрашивали подтверждение, игнорируйте это письмо.`;
  await sendMail(to, subject, text);
}

export async function sendPasswordResetEmail(to: string, code: string) {
  const subject = "Notepub: сброс пароля";
  const text = `Код для сброса пароля: ${code}\n\nКод действителен 15 минут. Введите его вместе с новым паролем на странице восстановления.`;
  await sendMail(to, subject, text);
}
