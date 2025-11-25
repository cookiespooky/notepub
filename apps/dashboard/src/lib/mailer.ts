import nodemailer from "nodemailer";
import SMTPTransport from "nodemailer/lib/smtp-transport";
import { setDefaultResultOrder } from "node:dns";
import { lookup } from "node:dns/promises";
import net from "node:net";
import { loadEnv } from "@notepub/env";

// Prefer IPv6 during DNS resolution to work around IPv4 egress blocks
setDefaultResultOrder("ipv6first");

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

const transportOptions: SMTPTransport.Options = {
  host: env.MAIL_HOST,
  port: env.MAIL_PORT || 465,
  secure: parseSecureFlag(),
  auth: {
    user: env.MAIL_USER,
    pass: env.MAIL_PASS,
  },
  tls: {
    servername: env.MAIL_HOST,
  },
  // Force IPv6 socket to avoid blocked IPv4 egress
  getSocket: (_opts, cb) => {
    createIpv6Socket(env.MAIL_HOST || "", env.MAIL_PORT || 465)
      .then((socket) => cb(null, { socket }))
      .catch((err) => cb(err as Error, undefined as any));
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

async function createIpv6Socket(host: string, port: number) {
  const { address } = await lookup(host, { family: 6 });
  return net.connect({
    host: address,
    port,
    family: 6,
    // ensure Node does not fall back to IPv4 (Happy Eyeballs)
    autoSelectFamily: false,
  });
}
