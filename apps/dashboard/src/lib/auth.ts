import bcrypt from "bcryptjs";
import { PrismaClientKnownRequestError } from "@prisma/client/runtime/library";
import { findUserWithPasswordByEmail, createUser } from "@notepub/core";
import { prisma } from "@notepub/db";

export async function hashPassword(password: string) {
  const saltRounds = 10;
  return bcrypt.hash(password, saltRounds);
}

export async function verifyPassword(password: string, hash: string) {
  return bcrypt.compare(password, hash);
}

export async function registerUser(email: string, password: string) {
  const hashed = await hashPassword(password);
  const user = await createUser(email);
  await prisma.password.create({
    data: {
      userId: user.id,
      hash: hashed,
    },
  });
  return user;
}

export async function authenticate(email: string, password: string) {
  const user = await findUserWithPasswordByEmail(email);
  if (!user || !user.password?.hash) return null;
  const ok = await verifyPassword(password, user.password.hash);
  if (!ok) return null;
  return user;
}

export async function setUserPassword(userId: string, password: string) {
  const hashed = await hashPassword(password);
  await prisma.password.upsert({
    where: { userId },
    update: { hash: hashed },
    create: { userId, hash: hashed },
  });
}

export function isUniqueConstraintError(error: unknown, field?: string) {
  if (!(error instanceof PrismaClientKnownRequestError)) return false;
  if (error.code !== "P2002") return false;
  if (!field) return true;
  const metaFields = (error.meta?.target as string[]) || [];
  return metaFields.includes(field);
}
