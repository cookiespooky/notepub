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

type SlugLookup = {
  byPath: Map<string, string>;
  byName: Map<string, string[]>;
  byAlias: Map<string, string[]>;
  folderIndexByName: Map<string, string[]>;
};

type AssetOpts = {
  objectKey: string;
  folderMeta?: Map<string, FolderMeta>;
  s3Prefix?: string;
  slugLookup?: SlugLookup;
};

const VIDEO_EXTS = new Set([".mp4", ".webm", ".ogg", ".mov"]);
const VIDEO_TYPES: Record<string, string> = {
  ".mp4": "video/mp4",
  ".webm": "video/webm",
  ".ogg": "video/ogg",
  ".mov": "video/quicktime",
};

export async function renderMarkdown(markdown: string, opts: AssetOpts) {
  const processor = unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkNotepubBlocks)
    .use(remarkCallouts)
    .use(remarkObsidianInline)
    .use(remarkSoftBreaks)
    .use(remarkExtractImageBlocks)
    .use(remarkBacklinks, {
      objectKey: opts.objectKey,
      folderMeta: opts.folderMeta,
      s3Prefix: opts.s3Prefix,
      slugLookup: opts.slugLookup,
    })
    .use(remarkVideoEmbeds)
    .use(remarkRehype, { allowDangerousHtml: true })
    .use(rehypeAssetUrls, { objectKey: opts.objectKey, s3Prefix: opts.s3Prefix, slugLookup: opts.slugLookup })
    .use(rehypeImageCaptions)
    .use(rehypeStripVideoBrackets)
    .use(rehypeExternalLinks)
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
  slugLookup?: SlugLookup;
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
          const resolved = resolveBacklink(targetPath.trim(), anchor?.trim(), options.objectKey, {
            folderMeta: options.folderMeta,
            s3Prefix: options.s3Prefix,
            slugLookup: options.slugLookup,
          });
          const linkNode: MdastLink = {
            type: "link",
            url: resolved.href || "#",
            data: resolved.slug ? { hProperties: { "data-note-slug": resolved.slug } } : undefined,
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

function remarkExtractImageBlocks() {
  return (tree: MdastRoot) => {
    visit(tree, "paragraph", (node: Paragraph, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const parts: (Paragraph | MdastImage)[] = [];
      let current: Paragraph | null = null;

      const flush = () => {
        if (current && current.children.length > 0) {
          parts.push(current);
        }
        current = null;
      };

      for (const child of node.children) {
        if (child.type === "image") {
          flush();
          parts.push(child as MdastImage);
        } else {
          if (!current) current = { type: "paragraph", children: [] };
          current.children.push(child as any);
        }
      }
      flush();

      if (parts.length > 1 || (parts.length === 1 && parts[0] !== node)) {
        (parent.children as any[]).splice(index, 1, ...parts);
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

type FormField = {
  name: string;
  label: string;
  type: "text" | "email" | "phone" | "textarea" | "privacy";
  required: boolean;
  href?: string;
};

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
  const submitRaw = Array.isArray(parsed.submit) ? parsed.submit[0] : parsed.submit;
  const submitText =
    typeof submitRaw?.text === "string" && submitRaw.text.trim() ? submitRaw.text.trim() : "Отправить";
  const redirect =
    typeof submitRaw?.redirect === "string" && submitRaw.redirect.trim() ? submitRaw.redirect.trim() : "";
  const fields: any[] = Array.isArray(parsed.fields) ? parsed.fields : [];

  const cleanedFields = fields
    .map((f): FormField | null => {
      const name = typeof f.name === "string" ? f.name.trim() : "";
      const label = typeof f.label === "string" ? f.label.trim() : "";
      const typeRaw = typeof f.type === "string" ? f.type.trim().toLowerCase() : inferFieldType(name);
      const type = ["text", "email", "phone", "textarea", "privacy"].includes(typeRaw) ? typeRaw : "text";
      const required = f.required === true;
      const href = typeof f.href === "string" ? f.href.trim() : undefined;
      if (!name) return null;
      return { name, label, type: type as FormField["type"], required, href };
    })
    .filter((field): field is FormField => Boolean(field));

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
  <input type="hidden" name="__redirect" value="${escapeAttr(redirect)}" />
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
  return `<a class="np-button" href="${escapeAttr(href)}" target="_blank" rel="noreferrer">${escapeHtml(text)}</a>`;
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

function inferFieldType(name: string) {
  const n = name.trim().toLowerCase();
  if (n === "email") return "email";
  if (n === "phone" || n === "tel") return "phone";
  if (n === "comment") return "textarea";
  if (n === "privacy") return "privacy";
  if (n === "name") return "text";
  return "text";
}

function resolveBacklink(
  targetPath: string,
  anchor: string | undefined,
  objectKey: string,
  opts: { folderMeta?: Map<string, FolderMeta>; s3Prefix?: string; slugLookup?: SlugLookup },
) {
  const anchorPart = anchor ? `#${slugifySegment(anchor)}` : "";
  if (!targetPath) {
    return { href: anchor ? anchorPart : "#", slug: null };
  }
  const slug = resolveNoteSlug(targetPath, objectKey, opts);
  if (slug) {
    return { href: `/${slug}${anchorPart}`, slug };
  }
  return { href: anchor ? anchorPart : "#", slug: null };
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

// Support Obsidian inline highlight ==text== and comments %% comment %%
function remarkObsidianInline() {
  return (tree: MdastRoot) => {
    visit(tree, "text", (node: Text, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const value = node.value || "";
      const parts: PhrasingContent[] = [];
      const regex = /(%%[\s\S]*?%%)|==([\s\S]+?)==/g;
      let lastIndex = 0;
      let match: RegExpExecArray | null;

      while ((match = regex.exec(value)) !== null) {
        const [raw, comment, highlight] = match;
        if (match.index > lastIndex) {
          parts.push({ type: "text", value: value.slice(lastIndex, match.index) });
        }
        if (comment) {
          const inner = comment.slice(2, -2);
          parts.push({
            type: "html",
            value: `<span class="np-comment">${escapeHtml(inner)}</span>`,
          } as unknown as PhrasingContent);
        } else if (typeof highlight === "string") {
          parts.push({
            type: "emphasis",
            data: { hName: "mark" },
            children: [{ type: "text", value: highlight }],
          } as unknown as PhrasingContent);
        }
        lastIndex = match.index + raw.length;
      }

      if (parts.length > 0) {
        if (lastIndex < value.length) {
          parts.push({ type: "text", value: value.slice(lastIndex) });
        }
        (parent.children as any[]).splice(index, 1, ...parts);
      }
    });
  };
}

// Convert soft line breaks inside paragraphs into <br />
function remarkSoftBreaks() {
  return (tree: MdastRoot) => {
    visit(tree, "text", (node, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const value = node.value || "";
      if (!value.includes("\n")) return;
      const parts = value.split("\n");
      const result: PhrasingContent[] = [];
      parts.forEach((part, idx) => {
        if (part) result.push({ type: "text", value: part });
        if (idx !== parts.length - 1) {
          result.push({ type: "break" } as unknown as PhrasingContent);
        }
      });
      if (result.length > 0) {
        (parent.children as any[]).splice(index, 1, ...result);
      }
    });
  };
}

function remarkVideoEmbeds() {
  return (tree: MdastRoot) => {
    visit(tree, "text", (node, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const value = node.value || "";
      if (!value.includes("![")) return;
      const regex = /!\[\[\s*(https?:[^\]\s]+)\s*\]\]/g;
      const parts: PhrasingContent[] = [];
      let lastIndex = 0;
      let match: RegExpExecArray | null;
      while ((match = regex.exec(value)) !== null) {
        const before = value.slice(lastIndex, match.index);
        if (before) parts.push({ type: "text", value: before });
        const url = match[1];
        const ext = url.toLowerCase().split(".").pop() || "";
        const dotExt = ext ? `.${ext}` : "";
        if (VIDEO_EXTS.has(dotExt)) {
          const type = VIDEO_TYPES[dotExt] || "video/mp4";
          parts.push({
            type: "html",
            value: `<video class="np-video" controls preload="metadata" playsinline><source src="${escapeAttr(
              url,
            )}" type="${type}"></video>`,
          } as unknown as PhrasingContent);
        } else {
          parts.push({ type: "text", value: match[0] });
        }
        lastIndex = match.index + match[0].length;
      }
      if (parts.length === 0) return;
      if (lastIndex < value.length) {
        parts.push({ type: "text", value: value.slice(lastIndex) });
      }
      (parent.children as any[]).splice(index, 1, ...parts);
    });

    visit(tree, "image", (node: MdastImage, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const url = (node.url || "").trim();
      if (!/^https?:\/\//i.test(url)) return;
      const ext = url.toLowerCase().split(".").pop() || "";
      const dotExt = ext ? `.${ext}` : "";
      if (!VIDEO_EXTS.has(dotExt)) return;
      const type = VIDEO_TYPES[dotExt] || "video/mp4";
      const videoNode: any = {
        type: "html",
        value: `<video class="np-video" controls preload="metadata" playsinline><source src="${escapeAttr(
          url,
        )}" type="${type}"></video>`,
      };
      (parent.children as any[]).splice(index, 1, videoNode);
    });

    visit(tree, "link", (node: MdastLink, index, parent) => {
      if (!parent || typeof index !== "number") return;
      const href = (node.url || "").trim();
      if (!/^https?:\/\//i.test(href)) return;
      const ext = href.toLowerCase().split(".").pop() || "";
      const dotExt = ext ? `.${ext}` : "";
      if (!VIDEO_EXTS.has(dotExt)) return;
      const type = VIDEO_TYPES[dotExt] || "video/mp4";
      const videoNode: any = {
        type: "html",
        value: `<video class="np-video" controls preload="metadata" playsinline><source src="${escapeAttr(
          href,
        )}" type="${type}"></video>`,
      };
      (parent.children as any[]).splice(index, 1, videoNode);
    });
  };
}

function rehypeStripVideoBrackets() {
  return (tree: HastRoot) => {
    visit(tree, "element", (node: Element) => {
      if (!node.children || !Array.isArray(node.children)) return;
      node.children = node.children.filter((child: any) => {
        if (child.type !== "text") return true;
        const value = String(child.value || "").trim();
        if (value === "![[") return false;
        if (value === "]]") return false;
        if (value === "![]") return false;
        return true;
      });
    });
  };
}

function rehypeExternalLinks() {
  return (tree: HastRoot) => {
    visit(tree, "element", (node: Element) => {
      if (node.tagName !== "a") return;
      const href = typeof node.properties?.href === "string" ? node.properties.href : "";
      if (!href) return;
      const isExternal = /^https?:\/\//i.test(href) || href.startsWith("//");
      if (!isExternal) return;
      const props = node.properties || {};
      props.target = props.target || "_blank";
      props.rel = props.rel || "noreferrer";
      node.properties = props;
    });
  };
}

function rehypeAssetUrls(options: { objectKey: string; s3Prefix?: string; slugLookup?: SlugLookup }) {
  return async (tree: HastRoot) => {
    const tasks: Promise<void>[] = [];
    visit(tree, "element", (node: Element) => {
      const props = node.properties || {};
      if (node.tagName === "img" && props.src) {
        const src = String(props.src);
        if (isHttpUrl(src) || src.startsWith("data:")) return;
        tasks.push(
          resolveAsset(src, options.objectKey, options.s3Prefix).then((resolved) => {
            node.properties = { ...props, src: resolved };
          }),
        );
      }
      if (node.tagName === "a" && props.href) {
        const href = String(props.href);
        if (isHttpUrl(href) || href.startsWith("#") || href.startsWith("/")) return;
        const [targetPath, anchor] = href.split("#");
        const normalizedTarget = targetPath.endsWith(".md") ? targetPath.replace(/\.md$/i, "") : targetPath;
        const slug = resolveNoteSlug(normalizedTarget, options.objectKey, {
          s3Prefix: options.s3Prefix,
          slugLookup: options.slugLookup,
        });
        if (slug) {
          const anchorPart = anchor ? `#${slugifySegment(anchor)}` : "";
          node.properties = { ...props, href: `/${slug}${anchorPart}`, "data-note-slug": slug };
        }
      }
    });
    await Promise.all(tasks);
  };
}

function rehypeImageCaptions() {
  return (tree: HastRoot) => {
    visit(tree, "element", (node: Element, index, parent) => {
      if (!parent || typeof index !== "number") return;
      if (node.tagName !== "img") return;
      const alt = (node.properties?.alt as string | undefined)?.trim();
      const src = (node.properties?.src as string | undefined) || "";
      if (!alt) return;
      if (looksLikeFilename(alt, src)) return;

      const figure: Element = {
        type: "element",
        tagName: "figure",
        properties: { className: ["np-figure"] },
        children: [
          node,
          {
            type: "element",
            tagName: "figcaption",
            properties: { className: ["np-figcaption"] },
            children: [{ type: "text", value: alt }],
          },
        ],
      };
      (parent.children as any[]).splice(index, 1, figure);
    });
  };
}

function looksLikeFilename(text: string, src?: string) {
  const cleaned = text.trim();
  if (!cleaned) return false;
  const fileNameRegex = /^[^\s/]+\.(png|jpe?g|gif|svg|webp|bmp|tiff|avif)$/i;
  if (fileNameRegex.test(cleaned)) return true;
  if (src) {
    try {
      const url = new URL(src, "http://localhost");
      const base = url.pathname.split("/").pop() || "";
      const baseNoExt = base.replace(/\.[^.]+$/, "");
      const lower = cleaned.toLowerCase();
      if (lower === base.toLowerCase() || lower === baseNoExt.toLowerCase()) return true;
    } catch {
      // ignore parsing errors
    }
  }
  return false;
}

async function resolveAsset(input: string, objectKey: string, s3Prefix?: string) {
  // Handle .np-assets (editor uploads) and relative assets.
  const cleaned = input.replace(/\?.*$/, "");
  // If coming from /api/editor/assets/... just strip the prefix; it already points at the stored key.
  const apiPrefix = "/api/editor/assets/";
  if (cleaned.startsWith(apiPrefix)) {
    const key = withPrefix(cleaned.slice(apiPrefix.length).replace(/^\/+/, ""), s3Prefix);
    return getSignedObjectUrl(key);
  }
  if (cleaned.startsWith(".np-assets") || cleaned.startsWith("/.np-assets")) {
    const key = withPrefix(cleaned.replace(/^\/+/, ""), s3Prefix);
    return getSignedObjectUrl(key);
  }
  const baseDir = path.posix.dirname(objectKey);
  const normalized = path.posix.normalize(path.posix.join(baseDir, cleaned));
  const baseName = path.posix.basename(cleaned);
  const isBareName = !cleaned.includes("/") || cleaned === baseName;
  const candidates: string[] = [];
  if (isBareName) {
    // Prefer note-relative first, then legacy .np-assets, then vault root.
    candidates.push(normalized);
    candidates.push(`.np-assets/${baseName}`);
    candidates.push(baseName);
  } else {
    candidates.push(normalized);
  }
  // Return first candidate; signed URL generation does not check existence.
  for (const candidate of candidates) {
    if (!candidate) continue;
    return getSignedObjectUrl(withPrefix(candidate, s3Prefix));
  }
  return getSignedObjectUrl(withPrefix(normalized, s3Prefix));
}

function withPrefix(key: string, s3Prefix?: string) {
  const normalizedKey = key.replace(/^\/+/, "");
  if (!s3Prefix) return normalizedKey;
  const trimmed = s3Prefix.replace(/^\/+|\/+$/g, "");
  if (!trimmed) return normalizedKey;
  if (normalizedKey.startsWith(trimmed)) return normalizedKey;
  return `${trimmed}/${normalizedKey}`;
}

// rehypeVideoEmbeds removed: handled in remarkVideoEmbeds to avoid leftover markup

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

function stripVaultPrefix(segments: string[]) {
  if (segments.length >= 4 && segments[0] === "publishers" && segments[2] === "vaults") {
    return segments.slice(4);
  }
  return segments;
}

  function resolveNoteSlug(targetPath: string, objectKey: string, opts: { s3Prefix?: string; slugLookup?: SlugLookup }) {
  const lookup = opts.slugLookup;
  const cleaned = targetPath.replace(/\\/g, "/").replace(/\.md$/i, "");
  const isAbsolute = cleaned.startsWith("/");
  const normalizedTarget = cleaned.replace(/^\/+/, "");
  const currentDir = path.posix.dirname(stripPrefix(objectKey, opts.s3Prefix));
  const currentSegments = currentDir.split("/").filter(Boolean);
  const targetSegments = normalizedTarget.split("/").filter(Boolean);
  const candidates: string[] = [];

  const baseSegments = isAbsolute ? [] : currentSegments;
  for (let depth = baseSegments.length; depth >= 0; depth--) {
    const combined = [...baseSegments.slice(0, depth), ...targetSegments];
    const normalizedCandidate = path.posix.normalize(combined.join("/"));
    const candidateSegments = normalizedCandidate.split("/").filter(Boolean);
    const candidatePath = candidateSegments.join("/");
    if (!candidatePath) continue;
    const withExt = candidatePath.endsWith(".md") ? candidatePath : `${candidatePath}.md`;
    if (withExt) candidates.push(withExt);
  }

  if (lookup) {
    // 1) exact path matches
    for (const candidate of candidates) {
      const slug = lookup.byPath.get(candidate);
      if (slug) return slug;
    }

    const leaf = targetSegments.at(-1);
    if (leaf) {
      const nameKey = normalizeName(leaf);
      const matches: string[] = [];
      (lookup.byName.get(nameKey) || []).forEach((p) => matches.push(p));
      (lookup.byAlias.get(nameKey) || []).forEach((p) => matches.push(p));
      (lookup.folderIndexByName.get(nameKey) || []).forEach((p) => matches.push(p));

      if (matches.length > 0) {
        const scored = matches
          .map((pathKey) => ({ pathKey, score: proximityScore(pathKey, currentSegments) }))
          .sort((a, b) => b.score - a.score);
        for (const cand of scored) {
          const slug = lookup.byPath.get(cand.pathKey);
          if (slug) return slug;
        }
      }
    }
  }

  // No slug found in lookup.
  return null;
}

function proximityScore(pathKey: string, currentSegments: string[]) {
  const pathSegments = pathKey.split("/").filter(Boolean);
  let shared = 0;
  for (let i = 0; i < Math.min(pathSegments.length - 1, currentSegments.length); i++) {
    if (pathSegments[i] === currentSegments[i]) {
      shared++;
    } else {
      break;
    }
  }
  return -Math.abs(pathSegments.length - 1 - currentSegments.length) + shared * 2;
}

function normalizeName(input: string) {
  return input.trim().toLowerCase();
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
