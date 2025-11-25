import path from "path";
import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkRehype from "remark-rehype";
import rehypeSlug from "rehype-slug";
import rehypeHighlight from "rehype-highlight";
import rehypeStringify from "rehype-stringify";
import { visit } from "unist-util-visit";
import { getSignedObjectUrl } from "./s3";
import { slugFromPathSegments, slugifySegment, resolveFolderSlugs } from "./slug";
import type { Root as MdastRoot, Blockquote, Paragraph, Text, Link as MdastLink, PhrasingContent, Image as MdastImage } from "mdast";
import type { Root as HastRoot, Element } from "hast";
import type { FolderMeta } from "./types";
import { getS3Config } from "./config";

type AssetOpts = {
  objectKey: string;
  folderMeta?: Map<string, FolderMeta>;
};

export async function markdownToHtml(markdown: string, opts: AssetOpts) {
  const processor = unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkCallouts)
    .use(remarkBacklinks, { objectKey: opts.objectKey, folderMeta: opts.folderMeta })
    .use(remarkRehype, { allowDangerousHtml: true })
    .use(rehypeAssetUrls, { objectKey: opts.objectKey })
    .use(rehypeSlug)
    .use(rehypeHighlight)
    .use(rehypeStringify, { allowDangerousHtml: true });

  const file = await processor.process(markdown);
  return String(file);
}

type BacklinkPluginOpts = {
  objectKey: string;
  folderMeta?: Map<string, FolderMeta>;
};

function remarkBacklinks(options: BacklinkPluginOpts) {
  return (tree: MdastRoot) => {
    visit(tree, "text", (node, index, parent) => {
      if (typeof index !== "number") return;
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
          const href = buildBacklinkHref(targetPath.trim(), anchor?.trim(), options.objectKey, options.folderMeta);
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

function buildBacklinkHref(targetPath: string, anchor: string | undefined, objectKey: string, folderMeta?: Map<string, FolderMeta>) {
  const anchorPart = anchor ? `#${slugifySegment(anchor)}` : "";
  if (!targetPath) {
    return anchor ? anchorPart : "#";
  }
  const normalizedLink = path.posix.normalize(path.posix.join(path.posix.dirname(objectKey), targetPath));
  const relative = stripPrefix(normalizedLink);
  let segments = stripConfiguredPrefixSegments(relative.split("/").filter(Boolean));
  segments = dedupeLeadingSegment(segments);
  if (segments.length === 0) return anchorPart || "#";
  const folderSegments = segments.slice(0, -1);
  const fileSegment = segments.at(-1)!;
  const folderSlugs = resolveFolderSlugs(folderSegments, folderMeta);
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

function rehypeAssetUrls(options: AssetOpts) {
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

function stripPrefix(key: string) {
  const { prefix } = getS3Config();
  if (!prefix) return key.replace(/^\/+/, "");
  const normalizedPrefix = prefix.replace(/^\/+/, "").replace(/\/+$/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedKey.startsWith(normalizedPrefix + "/")) {
    return normalizedKey.slice(normalizedPrefix.length + 1);
  }
  return normalizedKey;
}

function stripConfiguredPrefixSegments(segments: string[]) {
  const { prefix } = getS3Config();
  if (!prefix) return segments;
  const prefSegments = prefix.replace(/^\/+|\/+$/g, "").split("/").filter(Boolean);
  if (prefSegments.length === 0) return segments;
  const matches = prefSegments.every((seg, idx) => {
    const candidate = segments[idx];
    if (candidate === undefined) return false;
    return candidate === seg || slugifySegment(candidate) === slugifySegment(seg);
  });
  return matches ? segments.slice(prefSegments.length) : segments;
}

function isImageTarget(targetPath: string) {
  const ext = path.posix.extname(targetPath).toLowerCase();
  return [".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".avif"].includes(ext);
}

function dedupeLeadingSegment(segments: string[]) {
  if (segments.length < 2) return segments;
  const first = segments[0];
  const second = segments[1];
  if (slugifySegment(first) === slugifySegment(second)) {
    return segments.slice(1);
  }
  return segments;
}
