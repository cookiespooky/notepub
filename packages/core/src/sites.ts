import { prisma } from "@notepub/db";
import { putObjectString } from "@notepub/storage";

export async function getSiteBySlug(slug: string) {
  return prisma.site.findUnique({ where: { slug } });
}

export async function getSiteById(id: string) {
  return prisma.site.findUnique({ where: { id } });
}

export async function listSitesForUser(ownerId: string) {
  return prisma.site.findMany({
    where: { ownerId },
    orderBy: { createdAt: "desc" },
  });
}

export async function createSite(input: { slug: string; title: string; ownerId: string }) {
  const base = await prisma.site.create({
    data: {
      slug: input.slug,
      title: input.title,
      ownerId: input.ownerId,
      s3Prefix: "",
    },
  });

  const s3Prefix = `sites/${base.id}/vault`;
  if (base.s3Prefix === s3Prefix) {
    return base;
  }

  const updated = await prisma.site.update({
    where: { id: base.id },
    data: { s3Prefix },
  });

  try {
    await seedDefaultContent(s3Prefix);
  } catch (error) {
    console.error("Failed to create S3 prefix marker", error);
  }

  return updated;
}

function normalizePrefix(prefix: string) {
  const trimmed = prefix.replace(/^\/+/, "").replace(/\/+$/, "");
  return trimmed ? `${trimmed}/` : "";
}

async function seedDefaultContent(rawPrefix: string) {
  const prefix = normalizePrefix(rawPrefix);
  const root = `${prefix}`;
  const folderPath = `${prefix}ideas/`;

  const rootIndex = `---
title: Welcome
slug: welcome
---
# Welcome to your Notepub site

This is the default homepage. Edit \`index.md\` in your vault to replace this text.

- Your site slug: auto-generated
- Storage prefix: ${rawPrefix}
- Next steps: sync your Obsidian vault via Remotely Save to this folder.
`;

  const folderMeta = {
    title: "Ideas",
    slug: "ideas",
  };

  const folderIndex = `---
title: First idea
slug: first-idea
---
# First idea

Write down something interesting here. You can add more notes to this folder.
`;

  await Promise.all([
    putObjectString(`${root}.keep`, "notepub init"),
    putObjectString(`${root}index.md`, rootIndex, "text/markdown"),
    putObjectString(`${folderPath}_folder.json`, JSON.stringify(folderMeta, null, 2), "application/json"),
    putObjectString(`${folderPath}index.md`, folderIndex, "text/markdown"),
  ]);
}

export async function updateSite(input: {
  id: string;
  ownerId: string;
  slug: string;
  title: string;
  ogImageUrl?: string | null;
  ogDescription?: string | null;
}) {
  const site = await prisma.site.findUnique({ where: { id: input.id } });
  if (!site || site.ownerId !== input.ownerId) {
    throw new Error("Forbidden");
  }
  return prisma.site.update({
    where: { id: input.id },
    data: {
      slug: input.slug,
      title: input.title,
      ogImageUrl: input.ogImageUrl,
      ogDescription: input.ogDescription,
    },
  });
}

export async function deleteSite(input: { id: string; ownerId: string }) {
  const site = await prisma.site.findUnique({ where: { id: input.id } });
  if (!site || site.ownerId !== input.ownerId) {
    throw new Error("Forbidden");
  }
  return prisma.site.delete({ where: { id: input.id } });
}
