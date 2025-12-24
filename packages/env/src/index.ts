import { z } from "zod";

export const envSchema = z.object({
  NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
  APP_ENV: z.enum(["local", "staging", "production"]).optional(),
  DATABASE_URL: z.string().url().optional(),
  S3_ENDPOINT: z.string().trim().optional(),
  S3_REGION: z.string().trim().optional(),
  S3_BUCKET: z.string().min(1, "S3_BUCKET is required"),
  S3_ACCESS_KEY: z.string().min(1, "S3_ACCESS_KEY is required"),
  S3_SECRET_KEY: z.string().min(1, "S3_SECRET_KEY is required"),
  S3_PREFIX: z.string().optional(),
  COOKIE_SECRET: z.string().min(16, "COOKIE_SECRET must be at least 16 chars"),
  SESSION_COOKIE_NAME: z.string().default("notepub_session"),
  COOKIE_DOMAIN: z.string().optional(),
  MAIL_HOST: z.string().optional(),
  MAIL_PORT: z.coerce.number().optional(),
  MAIL_USER: z.string().optional(),
  MAIL_PASS: z.string().optional(),
  MAIL_FROM: z.string().optional(),
  MAIL_SECURE: z
    .union([z.literal("true"), z.literal("false"), z.literal("1"), z.literal("0")])
    .optional(),
  MAIL_SOCKET_HOST: z.string().optional(),
  MAIL_FROM_LEADS: z.string().optional(),
  APP_URL: z.string().url().optional(),
  MULTISITE_ALLOWLIST: z.string().optional(),
});

export type AppEnv = z.infer<typeof envSchema>;
export type AppEnvName = "local" | "staging" | "production";

export function loadEnv(raw: NodeJS.ProcessEnv = process.env): AppEnv {
  const parsed = envSchema.safeParse(raw);
  if (!parsed.success) {
    const message = parsed.error.issues.map((issue) => `${issue.path.join(".")}: ${issue.message}`).join("; ");
    throw new Error(`Invalid environment variables: ${message}`);
  }
  return parsed.data;
}

export function resolveAppEnv(env: AppEnv): AppEnvName {
  if (env.APP_ENV) return env.APP_ENV;
  if (env.NODE_ENV === "production") return "production";
  return "local";
}

export function resolveS3Prefix(env: AppEnv): string {
  const appEnv = resolveAppEnv(env);
  const fallback = appEnv === "production" ? "prod/" : appEnv === "staging" ? "staging/" : "local/";
  const normalized = normalizePrefix(env.S3_PREFIX ?? fallback);
  if (!normalized && appEnv !== "local") {
    throw new Error("S3_PREFIX is required for staging/production environments");
  }
  return normalized;
}

export function normalizePrefix(input: string | null | undefined): string {
  if (!input) return "";
  let prefix = input.trim();
  prefix = prefix.replace(/^\/+/, "");
  if (prefix && !prefix.endsWith("/")) prefix = `${prefix}/`;
  return prefix;
}
