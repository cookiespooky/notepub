import nodemailer, { type TransportOptions } from "nodemailer";
import { loadEnv } from "@notepub/env";

const env = loadEnv();

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
  if (raw === "false" || raw === "0") return false;
  return true;
}

const smtpOptions: TransportOptions = {
  host: env.MAIL_HOST,
  port: env.MAIL_PORT || 465,
  secure: parseSecureFlag(),
  family: 6, // prefer IPv6 to avoid blocked IPv4 egress
  auth: {
    user: env.MAIL_USER,
    pass: env.MAIL_PASS,
  },
  // force IPv6 during DNS resolution (Node >= 18)
  dnsLookup: (hostname, _options, cb) => {
    return nodemailer.dns.resolve(hostname, { family: 6 }, cb);
  },
};

const transporter = nodemailer.createTransport(smtpOptions);

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
