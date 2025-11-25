const slugRegex = /^[a-z0-9]([a-z0-9-]{1,61}[a-z0-9])?$/;

export function normalizeSlug(input: string) {
  const slug = input.toLowerCase().replace(/[^a-z0-9-]/g, "-").replace(/-{2,}/g, "-").replace(/^-+|-+$/g, "");
  if (!slugRegex.test(slug)) return "";
  return slug;
}
