import nodemailer from "nodemailer";
import SMTPTransport from "nodemailer/lib/smtp-transport";
import { loadEnv } from "@notepub/env";

const env = loadEnv();
const smtpHost = env.MAIL_HOST || "smtp.mail.ru";
const smtpPort = env.MAIL_PORT || 587;

function ensureMailEnv() {
  const missing = [];
  if (!env.MAIL_HOST) missing.push("MAIL_HOST");
  if (!env.MAIL_PORT) missing.push("MAIL_PORT");
  if (!env.MAIL_USER) missing.push("MAIL_USER");
  if (!env.MAIL_PASS) missing.push("MAIL_PASS");
  if (!env.MAIL_FROM) missing.push("MAIL_FROM");
  if (missing.length > 0) {
    throw new Error(`Missing mail configuration: ${missing.join(", ")}`);
  }
}

function parseSecureFlag() {
  const raw = env.MAIL_SECURE;
  return raw === "true" || raw === "1";
}

const transportOptions: SMTPTransport.Options = {
  host: smtpHost,
  port: smtpPort,
  secure: parseSecureFlag(),
  auth: {
    user: env.MAIL_USER,
    pass: env.MAIL_PASS,
  },
  tls: {
    servername: smtpHost,
  },
};

const transporter = nodemailer.createTransport(transportOptions);

export async function sendMail(to: string, subject: string, text: string, html?: string) {
  ensureMailEnv();
  const from = env.MAIL_FROM || env.MAIL_USER || "no-reply@example.com";
  return transporter.sendMail({
    from,
    to,
    subject,
    text,
    html: html || text,
  });
}
