-- Add per-site quotas and bind sync tokens to a specific site/vault
ALTER TABLE "Site"
  ADD COLUMN "vaultQuotaBytes" BIGINT NOT NULL DEFAULT 10485760,
  ADD COLUMN "vaultUsedBytes" BIGINT NOT NULL DEFAULT 0;

ALTER TABLE "SyncToken"
  ADD COLUMN "siteId" TEXT,
  ADD CONSTRAINT "SyncToken_siteId_fkey" FOREIGN KEY ("siteId") REFERENCES "Site"("id") ON DELETE CASCADE ON UPDATE CASCADE;

CREATE INDEX "SyncToken_siteId_idx" ON "SyncToken"("siteId");
