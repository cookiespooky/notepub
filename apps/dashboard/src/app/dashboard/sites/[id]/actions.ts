"use server";

import { redirect } from "next/navigation";
import { isRedirectError } from "next/dist/client/components/redirect";
import { updateSite, deleteSite } from "@notepub/core";
import { getCurrentUser } from "@/lib/session";
import { isUniqueConstraintError } from "@/lib/auth";
import { normalizeSlug } from "../new/helpers";

export async function updateSiteAction(
  prevState: { error?: string } | undefined,
  formData: FormData,
) {
  const user = await getCurrentUser();
  if (!user) {
    return { error: "Нужно войти" };
  }
  const id = (formData.get("id") || "").toString();
  const slugRaw = (formData.get("slug") || "").toString().trim();
  const title = (formData.get("title") || "").toString().trim();
  const slug = normalizeSlug(slugRaw);

  if (!id || !slug || !title) {
    return { error: "Slug и название обязательны" };
  }
  const ogImageUrl = (formData.get("ogImageUrl") || "").toString().trim() || null;
  const ogDescription = (formData.get("ogDescription") || "").toString().trim() || null;

  try {
    await updateSite({ id, ownerId: user.id, slug, title, ogImageUrl, ogDescription });
  } catch (err) {
    if (isRedirectError(err)) throw err;
    if (isUniqueConstraintError(err, "slug")) {
      return { error: "Такой slug уже занят" };
    }
    if (err instanceof Error && err.message === "Forbidden") {
      return { error: "Нет прав" };
    }
    return { error: "Не удалось обновить сайт" };
  }

  redirect(`/dashboard/sites/${id}`);
}

export async function deleteSiteAction(
  prevState: { error?: string } | undefined,
  formData: FormData,
) {
  const user = await getCurrentUser();
  if (!user) {
    return { error: "Нужно войти" };
  }
  const id = (formData.get("id") || "").toString();
  if (!id) return { error: "Некорректный запрос" };

  try {
    await deleteSite({ id, ownerId: user.id });
  } catch (err) {
    if (isRedirectError(err)) throw err;
    if (err instanceof Error && err.message === "Forbidden") {
      return { error: "Нет прав" };
    }
    return { error: "Не удалось удалить сайт" };
  }

  redirect("/dashboard/sites");
}
