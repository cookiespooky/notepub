"use server";

import { redirect } from "next/navigation";
import { isRedirectError } from "next/dist/client/components/redirect";
import { createSite, listSitesForUser } from "@notepub/core";
import { getCurrentUser } from "@/lib/session";
import { isUniqueConstraintError } from "@/lib/auth";
import { normalizeSlug } from "./helpers";

export async function createSiteAction(prevState: { error?: string } | undefined, formData: FormData) {
  const user = await getCurrentUser();
  if (!user) {
    return { error: "Нужно войти" };
  }
  if (!user.emailVerified) {
    return { error: "Подтвердите email, чтобы создать сайт" };
  }

  const existing = await listSitesForUser(user.id);
  if (existing.length >= 1) {
    return { error: "Лимит 1 сайт на пользователя" };
  }

  const slugRaw = (formData.get("slug") || "").toString().trim();
  const title = (formData.get("title") || "").toString().trim();
  const slug = normalizeSlug(slugRaw);

  if (!slug || !title) {
    return { error: "Slug и название обязательны" };
  }

  let site;
  try {
    site = await createSite({ slug, title, ownerId: user.id });
  } catch (err) {
    if (isRedirectError(err)) throw err;
    if (isUniqueConstraintError(err, "slug")) {
      return { error: "Такой slug уже занят" };
    }
    return { error: "Не удалось создать сайт" };
  }
  redirect(`/dashboard/sites/${site.id}`);
}
