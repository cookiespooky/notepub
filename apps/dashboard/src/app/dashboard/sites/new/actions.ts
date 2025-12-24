"use server";

import { createSite, listSitesForUser } from "@notepub/core";
import { getCurrentUser } from "@/lib/session";
import { isUniqueConstraintError } from "@/lib/auth";
import { normalizeSlug, isReservedSlug } from "./helpers";
import { canBypassSiteLimit } from "@/lib/limits";

export type CreateSiteState = { error?: string; success?: boolean; redirectTo?: string };

export async function createSiteAction(prevState: CreateSiteState | undefined, formData: FormData) {
  const user = await getCurrentUser();
  if (!user) {
    return { error: "Нужно войти" };
  }
  if (!user.emailVerified) {
    return { error: "Подтвердите email, чтобы создать сайт" };
  }

  const existing = await listSitesForUser(user.id);
  const canCreateMore = canBypassSiteLimit(user.email) || existing.length < 1;
  if (!canCreateMore) {
    return { error: "Лимит 1 сайт на пользователя" };
  }

  const slugRaw = (formData.get("slug") || "").toString().trim();
  const title = (formData.get("title") || "").toString().trim();
  const slug = normalizeSlug(slugRaw);

  if (!slug || !title) {
    return { error: "Slug и название обязательны" };
  }
  if (isReservedSlug(slug)) {
    return { error: "Такой slug зарезервирован" };
  }

  let site;
  try {
    site = await createSite({ slug, title, ownerId: user.id });
  } catch (err) {
    if (isUniqueConstraintError(err, "slug")) {
      return { error: "Такой slug уже занят" };
    }
    return { error: "Не удалось создать сайт" };
  }
  return { success: true, redirectTo: `/dashboard/sites/${site.id}` };
}
