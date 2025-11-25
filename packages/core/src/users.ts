import { prisma } from "@notepub/db";

export async function findUserByEmail(email: string) {
  return prisma.user.findUnique({ where: { email } });
}

export async function findUserWithPasswordByEmail(email: string) {
  return prisma.user.findUnique({
    where: { email },
    include: { password: true },
  });
}

export async function getUserById(id: string) {
  return prisma.user.findUnique({ where: { id } });
}

export async function createUser(email: string) {
  return prisma.user.create({ data: { email } });
}
