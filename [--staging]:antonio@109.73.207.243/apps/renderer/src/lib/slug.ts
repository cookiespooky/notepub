const cyrillicMap: Record<string, string> = {
  а: "a",
  б: "b",
  в: "v",
  г: "g",
  д: "d",
  е: "e",
  ё: "e",
  ж: "zh",
  з: "z",
  и: "i",
  й: "y",
  к: "k",
  л: "l",
  м: "m",
  н: "n",
  о: "o",
  п: "p",
  р: "r",
  с: "s",
  т: "t",
  у: "u",
  ф: "f",
  х: "kh",
  ц: "ts",
  ч: "ch",
  ш: "sh",
  щ: "shch",
  ы: "y",
  э: "e",
  ю: "yu",
  я: "ya",
};

function transliterate(input: string) {
  return input
    .split("")
    .map((char) => {
      const lower = char.toLowerCase();
      if (cyrillicMap[lower]) {
        const mapped = cyrillicMap[lower];
        return char === lower ? mapped : mapped.charAt(0).toUpperCase() + mapped.slice(1);
      }
      return char;
    })
    .join("");
}

export function slugifySegment(value: string) {
  const transliterated = transliterate(value);
  return transliterated
    .replace(/[^\p{L}\p{N}\s\-_]+/gu, "")
    .trim()
    .replace(/[\s_]+/g, "-")
    .toLowerCase();
}

export function resolveFolderSlugs(segments: string[], folderMeta?: Map<string, { slug?: string }>): string[] | null {
  const slugs: string[] = [];
  for (let i = 0; i < segments.length; i++) {
    const path = segments.slice(0, i + 1).join("/");
    const meta = folderMeta?.get(path);
    if (meta?.slug && meta.slug.trim().length > 0) {
      slugs.push(meta.slug.trim());
    } else {
      return null;
    }
  }
  return slugs;
}
