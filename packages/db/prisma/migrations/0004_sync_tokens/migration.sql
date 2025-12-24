-- Create SyncToken table for per-user app passwords
CREATE TABLE "SyncToken" (
  "id" TEXT NOT NULL,
  "userId" TEXT NOT NULL,
  "tokenHash" TEXT NOT NULL,
  "label" TEXT,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "lastUsedAt" TIMESTAMP(3),
  "isActive" BOOLEAN NOT NULL DEFAULT TRUE,
  CONSTRAINT "SyncToken_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "SyncToken_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX "SyncToken_userId_idx" ON "SyncToken"("userId");
CREATE INDEX "SyncToken_isActive_idx" ON "SyncToken"("isActive");
