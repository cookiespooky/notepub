import crypto from "crypto";
import { prisma } from "@notepub/db";
import { hashPassword } from "./auth";

const VERIFICATION_TTL_MS = 15 * 60 * 1000;
const RESET_TTL_MS = 15 * 60 * 1000;

function generateVerificationToken() {
  return crypto.randomBytes(16).toString("hex");
}

function generateResetCode() {
  return Math.floor(100000 + Math.random() * 900000).toString();
}

export async function issueVerificationCode(userId: string) {
  const code = generateVerificationToken();
  const expiresAt = new Date(Date.now() + VERIFICATION_TTL_MS);
  await prisma.emailVerificationCode.create({
    data: {
      code,
      userId,
      expiresAt,
    },
  });
  return code;
}

export async function consumeVerificationCode(userId: string, code: string) {
  const record = await prisma.emailVerificationCode.findFirst({
    where: {
      userId,
      code,
      usedAt: null,
      expiresAt: { gt: new Date() },
    },
    orderBy: { createdAt: "desc" },
  });
  if (!record) return false;
  await prisma.$transaction([
    prisma.emailVerificationCode.update({ where: { id: record.id }, data: { usedAt: new Date() } }),
    prisma.user.update({ where: { id: userId }, data: { emailVerified: true } }),
  ]);
  return true;
}

export async function issueResetToken(userId: string) {
  const token = generateResetCode();
  const expiresAt = new Date(Date.now() + RESET_TTL_MS);
  await prisma.passwordResetToken.create({
    data: {
      token,
      userId,
      expiresAt,
    },
  });
  return token;
}

export async function consumeResetToken(token: string, newPassword: string) {
  const record = await prisma.passwordResetToken.findFirst({
    where: {
      token,
      usedAt: null,
      expiresAt: { gt: new Date() },
    },
    orderBy: { createdAt: "desc" },
  });
  if (!record) return false;

  const hashed = await hashPassword(newPassword);

  await prisma.$transaction([
    prisma.passwordResetToken.update({ where: { id: record.id }, data: { usedAt: new Date() } }),
    prisma.password.upsert({
      where: { userId: record.userId },
      update: { hash: hashed },
      create: { userId: record.userId, hash: hashed },
    }),
  ]);

  return true;
}
