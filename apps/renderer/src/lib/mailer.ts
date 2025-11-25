import nodemailer from "nodemailer";
import { loadEnv } from "@notepub/env";

const env = loadEnv();

function parseSecureFlag() {
  const raw = env.MAIL_SECURE;
  if (raw === "false" || raw === "0") return false;
  return true;
}

const transporter = nodemailer.createTransport({
  host: env.MAIL_HOST,
  port: env.MAIL_PORT || 465,
  secure: parseSecureFlag(),
  auth: env.MAIL_USER
    ? {
        user: env.MAIL_USER,
        pass: env.MAIL_PASS,
      }
    : undefined,
});

export async function sendLeadEmail(to: string, subject: string, text: string) {
  const from = env.MAIL_FROM_LEADS || env.MAIL_FROM || env.MAIL_USER || "no-reply@example.com";
  return transporter.sendMail({
    from,
    to,
    subject,
    text,
  });
}
