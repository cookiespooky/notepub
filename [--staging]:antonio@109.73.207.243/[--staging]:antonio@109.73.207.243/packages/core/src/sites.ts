import { prisma } from "@notepub/db";
import type { Prisma } from "@prisma/client";
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

  const s3Prefix = `publishers/${input.ownerId}/vaults/${base.id}`;
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

  const rootIndex = `---
title: Добро пожаловать
slug: welcome
home: true
---
# Добро пожаловать на ваш сайт на Notepub!

Все готово для ваших идей.

## Быстрый старт на Notepub: https://about.notepub.site/manuals/quick-start

После того, как вы синхронизируете ваш Obsidian с Notepub, здесь появится ваш контент вместо этой страницы.
`;

  await Promise.all([
    putObjectString(`${root}.keep`, "notepub init"),
    putObjectString(`${root}welcome.md`, rootIndex, "text/markdown"),
  ]);
}

export async function updateSite(input: {
  id: string;
  ownerId: string;
  slug: string;
  title: string;
  ogImageUrl?: string | null;
  ogDescription?: string | null;
  hideSidebarOnHome?: boolean;
}) {
  const site = await prisma.site.findUnique({ where: { id: input.id } });
  if (!site || site.ownerId !== input.ownerId) {
    throw new Error("Forbidden");
  }

  const data: Prisma.SiteUpdateInput = {
    slug: input.slug,
    title: input.title,
    ...(input.ogImageUrl !== undefined ? { ogImageUrl: input.ogImageUrl } : {}),
    ...(input.ogDescription !== undefined ? { ogDescription: input.ogDescription } : {}),
    ...(input.hideSidebarOnHome !== undefined ? { hideSidebarOnHome: input.hideSidebarOnHome } : {}),
  };

  return prisma.site.update({
    where: { id: input.id },
    data,
  });
}

export async function deleteSite(input: { id: string; ownerId: string }) {
  const site = await prisma.site.findUnique({ where: { id: input.id } });
  if (!site || site.ownerId !== input.ownerId) {
    throw new Error("Forbidden");
  }

  // Manually cascade delete linked data to avoid FK restriction errors.
  await prisma.formSubmission.deleteMany({ where: { siteId: input.id } });

  return prisma.site.delete({ where: { id: input.id } });
}
