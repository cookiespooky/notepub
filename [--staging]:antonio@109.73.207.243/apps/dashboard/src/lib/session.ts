import crypto from "crypto";
import { cookies } from "next/headers";
import { loadEnv } from "@notepub/env";
import { getUserById } from "@notepub/core";
import type { User } from "@prisma/client";

const env = loadEnv();
const COOKIE_NAME = env.SESSION_COOKIE_NAME;
const SECRET = env.COOKIE_SECRET;
const IS_PROD = env.NODE_ENV === "production";

type CurrentUser = User & { emailVerified: boolean };

type SessionPayload = {
  userId: string;
  exp: number;
  emailVerified?: boolean;
};

function sign(payload: string) {
  return crypto.createHmac("sha256", SECRET).update(payload).digest("hex");
}

function encodeSession(session: SessionPayload) {
  const payload = Buffer.from(JSON.stringify(session)).toString("base64url");
  const signature = sign(payload);
  return `${payload}.${signature}`;
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

export async function getCurrentUser(): Promise<CurrentUser | null> {
  const sessionCookie = cookies().get(COOKIE_NAME)?.value;
  const session = decodeSession(sessionCookie);
  if (!session) return null;
  const user = await getUserById(session.userId);
  if (!user) return null;
  const emailVerified = (user as Partial<User>).emailVerified ?? false;
  return { ...user, emailVerified };
}

export function setSession(userId: string, emailVerified = false) {
  const maxAgeDays = 30;
  const exp = Date.now() + maxAgeDays * 24 * 60 * 60 * 1000;
  const value = encodeSession({ userId, exp, emailVerified });
  cookies().set({
    name: COOKIE_NAME,
    value,
    httpOnly: true,
    sameSite: "lax",
    secure: IS_PROD,
    path: "/",
    maxAge: maxAgeDays * 24 * 60 * 60,
  });
}

export function clearSession() {
  cookies().set({
    name: COOKIE_NAME,
    value: "",
    httpOnly: true,
    sameSite: "lax",
    secure: IS_PROD,
    path: "/",
    maxAge: 0,
  });
}

export function setSessionVerified(emailVerified: boolean) {
  const sessionCookie = cookies().get(COOKIE_NAME)?.value;
  const session = decodeSession(sessionCookie);
  if (!session) return;
  const maxAgeDays = 30;
  const exp = Date.now() + maxAgeDays * 24 * 60 * 60 * 1000;
  const value = encodeSession({ userId: session.userId, exp, emailVerified });
  cookies().set({
    name: COOKIE_NAME,
    value,
    httpOnly: true,
    sameSite: "lax",
    secure: IS_PROD,
    path: "/",
    maxAge: maxAgeDays * 24 * 60 * 60,
  });
}

export function isEmailVerified() {
  const sessionCookie = cookies().get(COOKIE_NAME)?.value;
  const session = decodeSession(sessionCookie);
  return session?.emailVerified === true;
}
