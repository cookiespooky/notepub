-- Add submissions table and relations
CREATE TABLE "FormSubmission" (
    "id" TEXT NOT NULL,
    "siteId" TEXT NOT NULL,
    "ownerId" TEXT NOT NULL,
    "formId" TEXT NOT NULL,
    "formTitle" TEXT,
    "payload" JSONB NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "FormSubmission_pkey" PRIMARY KEY ("id")
);

-- Indexes
CREATE INDEX "FormSubmission_siteId_idx" ON "FormSubmission"("siteId");
CREATE INDEX "FormSubmission_ownerId_idx" ON "FormSubmission"("ownerId");
CREATE INDEX "FormSubmission_formId_idx" ON "FormSubmission"("formId");

-- FKs
ALTER TABLE "FormSubmission" ADD CONSTRAINT "FormSubmission_siteId_fkey" FOREIGN KEY ("siteId") REFERENCES "Site"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
ALTER TABLE "FormSubmission" ADD CONSTRAINT "FormSubmission_ownerId_fkey" FOREIGN KEY ("ownerId") REFERENCES "User"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
