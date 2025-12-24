import bcrypt from "bcryptjs";
import crypto from "crypto";
import { prisma } from "@notepub/db";

export type SyncTokenRecord = {
  id: string;
  userId: string;
  siteId: string | null;
  label: string | null;
  createdAt: Date;
  lastUsedAt: Date | null;
  isActive: boolean;
};

export type SyncAuthedUser = {
  id: string;
  email: string;
  siteId: string;
  quotaBytes: bigint;
  usedBytes: bigint;
};

const SALT_ROUNDS = 12;

export async function createSyncToken(userId: string, siteId: string, label?: string | null) {
  const raw = generateToken();
  const tokenHash = await bcrypt.hash(raw, SALT_ROUNDS);

  const record = await prisma.syncToken.create({
    data: { userId, siteId, label: label || null, tokenHash },
  });

  return { token: raw, record: toRecord(record) };
}

export async function listSyncTokens(userId: string, siteId?: string): Promise<SyncTokenRecord[]> {
  const rows = await prisma.syncToken.findMany({
    where: { userId, siteId: siteId || undefined },
    orderBy: { createdAt: "desc" },
  });
  return rows.map(toRecord);
}

export async function setSyncTokenActive(id: string, userId: string, isActive: boolean, siteId?: string) {
  const row = await prisma.syncToken.findUnique({ where: { id } });
  if (!row || row.userId !== userId) throw new Error("Forbidden");
  if (siteId && row.siteId !== siteId) throw new Error("Forbidden");
  await prisma.syncToken.update({
    where: { id },
    data: { isActive },
  });
}

export async function findUserByToken(username: string, rawToken: string): Promise<SyncAuthedUser | null> {
  const user = await prisma.user.findUnique({
    where: { email: username },
    select: { id: true, email: true },
  });
  if (!user) return null;
  const tokens = await prisma.syncToken.findMany({
    where: { userId: user.id, isActive: true },
    include: {
      site: { select: { id: true, vaultQuotaBytes: true, vaultUsedBytes: true, s3Prefix: true } },
    },
  });
  for (const token of tokens) {
    const ok = await bcrypt.compare(rawToken, token.tokenHash);
    if (ok) {
      await prisma.syncToken.update({
        where: { id: token.id },
        data: { lastUsedAt: new Date() },
      });
      if (!token.site) return null;
      return {
        id: user.id,
        email: user.email,
        siteId: token.site.id,
        quotaBytes: token.site.vaultQuotaBytes ?? BigInt(10 * 1024 * 1024),
        usedBytes: token.site.vaultUsedBytes ?? BigInt(0),
      };
    }
  }
  return null;
}

function generateToken() {
  const buf = crypto.randomBytes(24);
  return buf.toString("base64url");
}

function toRecord(row: {
  id: string;
  userId: string;
  siteId: string | null;
  label: string | null;
  createdAt: Date;
  lastUsedAt: Date | null;
  isActive: boolean;
}) {
  return {
    id: row.id,
    userId: row.userId,
    siteId: row.siteId,
    label: row.label,
    createdAt: row.createdAt,
    lastUsedAt: row.lastUsedAt,
    isActive: row.isActive,
  };
}
