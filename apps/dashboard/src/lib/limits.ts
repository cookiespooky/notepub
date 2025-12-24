import { loadEnv } from "@notepub/env";

const env = loadEnv();

function parseAllowlist(raw?: string | null): Set<string> {
  if (!raw) return new Set();
  return new Set(
    raw
      .split(",")
      .map((item) => item.trim().toLowerCase())
      .filter(Boolean),
  );
}

const multisiteAllow = parseAllowlist(env.MULTISITE_ALLOWLIST);

export function canBypassSiteLimit(email: string | null | undefined): boolean {
  if (!email) return false;
  return multisiteAllow.has(email.trim().toLowerCase());
}
