-- Add per-user quota tracking (defaults: 10MB quota, 0 used)
ALTER TABLE "User"
  ADD COLUMN "quotaBytes" BIGINT NOT NULL DEFAULT 10485760,
  ADD COLUMN "usedBytes" BIGINT NOT NULL DEFAULT 0;
