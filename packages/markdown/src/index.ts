import path from "path";
import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkRehype from "remark-rehype";
import rehypeSlug from "rehype-slug";
import rehypeHighlight from "rehype-highlight";
import rehypeStringify from "rehype-stringify";
import { visit } from "unist-util-visit";
import yaml from "js-yaml";
import type {
  Root as MdastRoot,
  Blockquote,
  Paragraph,
  Text,
  Link as MdastLink,
  PhrasingContent,
  Image as MdastImage,
  Code,
} from "mdast";
import type { Root as HastRoot, Element } from "hast";
import { getSignedObjectUrl } from "@notepub/storage";

export type FolderMeta = {
  path: string;
  title?: string;
  slug?: string;
  etag?: string;
  renderVersion?: string;
};

type AssetOpts = {
  objectKey: string;
  folderMeta?: Map<string, FolderMeta>;
  s3Prefix?: string;
};

export async function renderMarkdown(markdown: string, opts: AssetOpts) {
  const processor = unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkNotepubBlocks)
    .use(remarkCallouts)
    .use(remarkBacklinks, { objectKey: opts.objectKey, folderMeta: opts.folderMeta, s3Prefix: opts.s3Prefix })
    .use(remarkRehype, { allowDangerousHtml: true })
    .use(rehypeAssetUrls, { objectKey: opts.objectKey, s3Prefix: opts.s3Prefix })
    .use(rehypeSlug)
    .use(rehypeHighlight)
    .use(rehypeStringify, { allowDangerousHtml: true });

  const file = await processor.process(markdown);
  return String(file);
}

type BacklinkPluginOpts = {
  objectKey: string;
  folderMeta?: Map<string, FolderMeta>;
  s3Prefix?: string;
};

function remarkBacklinks(options: BacklinkPluginOpts) {
  return (tree: MdastRoot) => {
    visit(tree, "text", (node, index, parent) => {
      if (!parent || !Array.isArray(parent.children)) return;
      const value = node.value || "";
      const regex = /(!)?\[\[([^[\]]+)\]\]/g;
      const newChildren: PhrasingContent[] = [];
      let lastIndex = 0;

      let match: RegExpExecArray | null;
      while ((match = regex.exec(value)) !== null) {
        const chunkBefore = value.slice(lastIndex, match.index);
        const isEmbed = !!match[1];
        const raw = match[2];
        const [targetPart, aliasPart] = raw.split("|");
        const [targetPath, anchor] = targetPart.split("#");
        const imageTarget = isImageTarget(targetPath);

        if (chunkBefore) {
          const cleaned = imageTarget || isEmbed ? chunkBefore.replace(/!?\s*$/, "") : chunkBefore;
          if (cleaned) newChildren.push({ type: "text", value: cleaned });
        }

        if (isEmbed || imageTarget) {
          const src = targetPath.trim();
          const alt = aliasPart?.trim() || targetPath.trim();
          const imageNode: MdastImage = {
            type: "image",
            url: src,
            alt,
          };
          newChildren.push(imageNode as unknown as PhrasingContent);
        } else {
          const label = aliasPart?.trim() || targetPath.trim() || raw;
          const href = buildBacklinkHref(targetPath.trim(), anchor?.trim(), options.objectKey, {
            folderMeta: options.folderMeta,
            s3Prefix: options.s3Prefix,
          });
          const linkNode: MdastLink = {
            type: "link",
            url: href || "#",
            children: [{ type: "text", value: label }],
          };
          newChildren.push(linkNode);
        }
        lastIndex = match.index + match[0].length;
      }

      if (newChildren.length > 0) {
        if (lastIndex < value.length) {
          newChildren.push({ type: "text", value: value.slice(lastIndex) });
        }
        const idx = typeof index === "number" ? index : 0;
        const children = parent.children as unknown as any[];
        children.splice(idx, 1, ...newChildren);
      }
    });
  };
}

function remarkNotepubBlocks() {
  return (tree: MdastRoot) => {
    visit(tree, "code", (node: Code, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const lang = (node.lang || "").trim().toLowerCase();
      if (lang === "notepub-form") {
        const html = renderFormBlock(node.value || "");
        if (html) {
          (parent.children as any[]).splice(index, 1, { type: "html", value: html });
        }
      } else if (lang === "notepub-button") {
        const html = renderButtonBlock(node.value || "");
        if (html) {
          (parent.children as any[]).splice(index, 1, { type: "html", value: html });
        }
      }
    });
  };
}

function renderFormBlock(raw: string) {
  let parsed: any;
  try {
    parsed = yaml.load(raw) || {};
  } catch (error) {
    console.warn("Failed to parse notepub-form block", error);
    return null;
  }
  if (!parsed || typeof parsed !== "object") return null;
  const formId = typeof parsed.id === "string" && parsed.id.trim() ? parsed.id.trim() : "form";
  const formTitle = typeof parsed.title === "string" ? parsed.title.trim() : "";
  const submitText =
    typeof parsed.submit?.text === "string" && parsed.submit.text.trim() ? parsed.submit.text.trim() : "Отправить";
  const fields: any[] = Array.isArray(parsed.fields) ? parsed.fields : [];

  const cleanedFields = fields
    .map((f) => {
      const name = typeof f.name === "string" ? f.name.trim() : "";
      const label = typeof f.label === "string" ? f.label.trim() : "";
      const typeRaw = typeof f.type === "string" ? f.type.trim().toLowerCase() : "text";
      const type = ["text", "email", "phone", "textarea", "privacy"].includes(typeRaw) ? typeRaw : "text";
      const required = f.required === true;
      const href = typeof f.href === "string" ? f.href.trim() : undefined;
      if (!name) return null;
      return { name, label, type, required, href };
    })
    .filter(Boolean);

  const schema = escapeAttr(JSON.stringify({ id: formId, title: formTitle, fields: cleanedFields }));

  const fieldsHtml = cleanedFields
    .map((field) => {
      const required = field.required ? " required" : "";
      const label = escapeHtml(field.label || field.name);
      const name = escapeAttr(field.name);
      const maxLen = ' maxlength="200"';
      if (field.type === "textarea") {
        return `<label class="np-field"><span>${label}</span><textarea name="${name}" rows="4"${maxLen}${required}></textarea></label>`;
      }
      if (field.type === "email") {
        return `<label class="np-field"><span>${label}</span><input type="email" name="${name}"${maxLen}${required} /></label>`;
      }
      if (field.type === "phone") {
        return `<label class="np-field"><span>${label}</span><input type="tel" name="${name}"${maxLen}${required} /></label>`;
      }
      if (field.type === "privacy") {
        const href = field.href ? ` href="${escapeAttr(field.href)}" target="_blank" rel="noreferrer"` : "";
        return `<label class="np-field np-privacy"><input type="checkbox" name="${name || "privacy"}"${required} /><span>${label}${
          field.href ? ` (<a${href}>политика</a>)` : ""
        }</span></label>`;
      }
      return `<label class="np-field"><span>${label}</span><input type="text" name="${name}"${maxLen}${required} /></label>`;
    })
    .join("");

  const titleHtml = formTitle ? `<div class="np-form-title">${escapeHtml(formTitle)}</div>` : "";

  return `<div class="np-form" data-notepub-form-shell>
${titleHtml}
<form class="np-form-body" data-notepub-form data-form-id="${escapeAttr(formId)}">
  <input type="hidden" name="formId" value="${escapeAttr(formId)}" />
  <input type="hidden" name="formTitle" value="${escapeAttr(formTitle)}" />
  <input type="hidden" name="__schema" value="${schema}" />
  ${fieldsHtml}
  <button type="submit" class="np-button">${escapeHtml(submitText)}</button>
  <div class="np-form-status" aria-live="polite"></div>
</form>
</div>`;
}

function renderButtonBlock(raw: string) {
  let parsed: any;
  try {
    parsed = yaml.load(raw) || {};
  } catch (error) {
    console.warn("Failed to parse notepub-button block", error);
    return null;
  }
  if (!parsed || typeof parsed !== "object") return null;
  const text = typeof parsed.text === "string" && parsed.text.trim() ? parsed.text.trim() : "Перейти";
  const href = typeof parsed.href === "string" && parsed.href.trim() ? parsed.href.trim() : "#";
  return `<a class="np-button" href="${escapeAttr(href)}">${escapeHtml(text)}</a>`;
}

function escapeHtml(value: string) {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function escapeAttr(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function buildBacklinkHref(
  targetPath: string,
  anchor: string | undefined,
  objectKey: string,
  opts: { folderMeta?: Map<string, FolderMeta>; s3Prefix?: string },
) {
  const anchorPart = anchor ? `#${slugifySegment(anchor)}` : "";
  if (!targetPath) {
    return anchor ? anchorPart : "#";
  }
  const normalizedLink = path.posix.normalize(path.posix.join(path.posix.dirname(objectKey), targetPath));
  const relative = stripPrefix(normalizedLink, opts.s3Prefix);
  let segments = stripConfiguredPrefixSegments(relative.split("/").filter(Boolean), opts.s3Prefix);
  segments = dedupeLeadingSegment(segments);
  if (segments.length === 0) return anchorPart || "#";
  const folderSegments = segments.slice(0, -1);
  const fileSegment = segments.at(-1)!;
  const folderSlugs = resolveFolderSlugs(folderSegments, opts.folderMeta);
  const slug = slugFromPathSegments([...folderSlugs, fileSegment]);
  return `/${slug}${anchorPart}`;
}

// rudimentary support for Obsidian callouts [!note]
function remarkCallouts() {
  return (tree: MdastRoot) => {
    visit(tree, "blockquote", (node: Blockquote) => {
      const firstChild = node.children?.[0] as Paragraph | undefined;
      if (!firstChild || firstChild.type !== "paragraph") return;

      const textNode = firstChild.children?.find((child) => child.type === "text") as Text | undefined;
      if (!textNode || typeof textNode.value !== "string") return;

      const match = textNode.value.match(/^\[!([^\]]+)\]\s*(.*)$/);
      if (!match) return;

      const kind = match[1].toLowerCase();
      const rest = match[2] || "";
      textNode.value = rest;
      // remove empty text nodes to keep markdown clean
      firstChild.children = firstChild.children.filter(
        (child) => child.type !== "text" || (child as Text).value.trim() !== "",
      );
      node.data = {
        ...(node.data || {}),
        hProperties: {
          className: ["callout", `callout-${kind}`],
          "data-callout": kind,
        },
      };
    });
  };
}

function rehypeAssetUrls(options: { objectKey: string; s3Prefix?: string }) {
  return async (tree: HastRoot) => {
    const tasks: Promise<void>[] = [];
    visit(tree, "element", (node: Element) => {
      const props = node.properties || {};
      if (node.tagName === "img" && props.src) {
        const src = String(props.src);
        if (isHttpUrl(src) || src.startsWith("data:")) return;
        tasks.push(
          resolveAsset(src, options.objectKey).then((resolved) => {
            node.properties = { ...props, src: resolved };
          }),
        );
      }
      if (node.tagName === "a" && props.href) {
        const href = String(props.href);
        if (isHttpUrl(href) || href.startsWith("#") || href.startsWith("/")) return;
        const targetPath = href.endsWith(".md") ? href.replace(/\.md$/i, "") : href;
        const normalizedLink = path.posix.normalize(path.posix.join(path.posix.dirname(options.objectKey), targetPath));
        const segments = normalizedLink.split("/").filter(Boolean);
        if (segments.length > 0) {
          const slug = slugFromPathSegments(segments);
          node.properties = { ...props, href: `/${slug}` };
        }
      }
    });
    await Promise.all(tasks);
  };
}

async function resolveAsset(input: string, objectKey: string) {
  const baseDir = path.posix.dirname(objectKey);
  const normalized = path.posix.normalize(path.posix.join(baseDir, input));
  return getSignedObjectUrl(normalized);
}

function isHttpUrl(candidate: string) {
  return /^https?:\/\//i.test(candidate);
}

function stripPrefix(key: string, s3Prefix?: string) {
  if (!s3Prefix) return key.replace(/^\/+/, "");
  const normalizedPrefix = normalizePrefix(s3Prefix).replace(/\/$/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedKey.startsWith(normalizedPrefix + "/")) {
    return normalizedKey.slice(normalizedPrefix.length + 1);
  }
  return normalizedKey;
}

function stripConfiguredPrefixSegments(segments: string[], s3Prefix?: string) {
  if (!s3Prefix) return segments;
  const prefSegments = normalizePrefix(s3Prefix)
    .replace(/\/$/, "")
    .split("/")
    .filter(Boolean);
  if (segments.length < prefSegments.length) return segments;
  if (prefSegments.every((seg, idx) => segments[idx] === seg)) {
    return segments.slice(prefSegments.length);
  }
  return segments;
}

function dedupeLeadingSegment(segments: string[]) {
  if (segments.length >= 2 && segments[0] === segments[1]) {
    return segments.slice(1);
  }
  return segments;
}

function isImageTarget(targetPath: string) {
  return /\.(png|jpe?g|gif|webp|svg)$/i.test(targetPath || "");
}

// slug helpers (copied from existing renderer logic)
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

export function slugFromPathSegments(segments: string[]) {
  return segments
    .map((segment) => slugifySegment(segment))
    .filter(Boolean)
    .join("/");
}

export function resolveFolderSlugs(segments: string[], folderMeta?: Map<string, { slug?: string }>) {
  const slugs: string[] = [];
  for (let i = 0; i < segments.length; i++) {
    const path = segments.slice(0, i + 1).join("/");
    const meta = folderMeta?.get(path);
    if (meta?.slug && meta.slug.trim().length > 0) {
      slugs.push(meta.slug.trim());
    } else {
      slugs.push(slugifySegment(segments[i]));
    }
  }
  return slugs;
}

function normalizePrefix(input: string) {
  if (!input) return "";
  let prefix = input.trim();
  prefix = prefix.replace(/^\/+/, "");
  if (prefix && !prefix.endsWith("/")) prefix = `${prefix}/`;
  return prefix;
}
