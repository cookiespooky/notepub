import crypto from "crypto";
import { cookies } from "next/headers";
import { loadEnv } from "@notepub/env";
import { prisma } from "@notepub/db";
import type { User } from "@prisma/client";

const env = loadEnv();
const COOKIE_NAME = env.SESSION_COOKIE_NAME;
const SECRET = env.COOKIE_SECRET;
const IS_PROD = env.NODE_ENV === "production";
const COOKIE_DOMAIN = env.COOKIE_DOMAIN;

type SessionPayload = {
  userId: string;
  exp: number;
  emailVerified?: boolean;
};

function sign(payload: string) {
  return crypto.createHmac("sha256", SECRET).update(payload).digest("hex");
}

function decodeSession(value: string | undefined | null): SessionPayload | null {
  if (!value) return null;
  const [payload, signature] = value.split(".");
  if (!payload || !signature) return null;
  if (sign(payload) !== signature) return null;
  try {
    const parsed = JSON.parse(Buffer.from(payload, "base64url").toString("utf8")) as SessionPayload;
    if (!parsed.userId || !parsed.exp) return null;
    if (Date.now() > parsed.exp) return null;
    return parsed;
  } catch {
    return null;
  }
}

export async function getCurrentUser(): Promise<(User & { emailVerified: boolean }) | null> {
  const sessionCookie = cookies().get(COOKIE_NAME)?.value;
  const session = decodeSession(sessionCookie);
  if (!session) return null;
  const user = await prisma.user.findUnique({ where: { id: session.userId } });
  if (!user) return null;
  const emailVerified = session.emailVerified === true || (user as any).emailVerified === true;
  return { ...user, emailVerified };
}

export function clearSession() {
  cookies().set({
    name: COOKIE_NAME,
    value: "",
    httpOnly: true,
    sameSite: "lax",
    secure: IS_PROD,
    path: "/",
    domain: COOKIE_DOMAIN,
    maxAge: 0,
  });
}
