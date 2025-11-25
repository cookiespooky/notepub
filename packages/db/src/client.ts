import { PrismaClient } from "@prisma/client";
import { loadEnv } from "@notepub/env";

const env = loadEnv();

if (!env.DATABASE_URL) {
  throw new Error("DATABASE_URL is required for Prisma client");
}

const globalForPrisma = globalThis as unknown as {
  prisma?: PrismaClient;
};

export const prisma =
  globalForPrisma.prisma ||
  new PrismaClient({
    datasources: {
      db: {
        url: env.DATABASE_URL,
      },
    },
  });

if (process.env.NODE_ENV !== "production") {
  globalForPrisma.prisma = prisma;
}
