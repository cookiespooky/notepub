import nodemailer from "nodemailer";
import SMTPTransport from "nodemailer/lib/smtp-transport";
import { loadEnv } from "@notepub/env";

const env = loadEnv();

const smtpHost = env.MAIL_HOST || "smtp.mail.ru";

function parsePort(raw: unknown, fallback: number): number {
  if (!raw) return fallback;
  const n = Number(raw);
  return Number.isFinite(n) ? n : fallback;
}

const smtpPort = parsePort(env.MAIL_PORT, 465);

function parseSecureFlag(raw: unknown, port: number): boolean {
  if (raw === "false" || raw === "0") return false;
  if (raw === "true" || raw === "1") return true;
  return port === 465;
}

const secure = parseSecureFlag(env.MAIL_SECURE, smtpPort);

function ensureMailEnv() {
  const missing = [];
  if (!env.MAIL_USER) missing.push("MAIL_USER");
  if (!env.MAIL_PASS) missing.push("MAIL_PASS");
  if (!env.MAIL_FROM) missing.push("MAIL_FROM");
  if (missing.length > 0) {
    throw new Error(`Missing mail configuration: ${missing.join(", ")}`);
  }
}

type SMTPOptions = ConstructorParameters<typeof SMTPTransport>[0];

const transportOptions: SMTPOptions = {
  host: smtpHost,
  port: smtpPort,
  secure,
  auth: {
    user: env.MAIL_USER,
    pass: env.MAIL_PASS,
  },
  connectionTimeout: 15000,
  greetingTimeout: 15000,
  socketTimeout: 20000,
};

const transporter = nodemailer.createTransport(transportOptions);

export async function sendLeadEmail(to: string, subject: string, text: string) {
  ensureMailEnv();
  const from = env.MAIL_FROM_LEADS || env.MAIL_FROM || env.MAIL_USER || "no-reply@example.com";
  return transporter.sendMail({
    from,
    to,
    subject,
    text,
    html: text,
  });
}
