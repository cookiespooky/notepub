-- Add optional OG fields for site metadata
ALTER TABLE "Site"
ADD COLUMN "ogImageUrl" TEXT,
ADD COLUMN "ogDescription" TEXT;
