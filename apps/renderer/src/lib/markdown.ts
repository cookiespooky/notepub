import { renderMarkdown } from "@notepub/markdown";
import { getS3Config } from "./config";

type AssetOpts = {
  objectKey: string;
  s3Prefix?: string;
  slugLookup?:
    | Map<string, string>
    | {
        byPath: Map<string, string>;
        byName: Map<string, string[]>;
        byAlias: Map<string, string[]>;
        folderIndexByName: Map<string, string[]>;
      };
};

export async function markdownToHtml(markdown: string, opts: AssetOpts) {
  const { prefix: basePrefix } = getS3Config();
  const s3Prefix = opts.s3Prefix ?? basePrefix;
  const slugLookup =
    opts.slugLookup && "byPath" in opts.slugLookup
      ? opts.slugLookup
      : opts.slugLookup
        ? {
            byPath: opts.slugLookup,
            byName: new Map<string, string[]>(),
            byAlias: new Map<string, string[]>(),
            folderIndexByName: new Map<string, string[]>(),
          }
        : undefined;
  return renderMarkdown(markdown, { objectKey: opts.objectKey, s3Prefix, slugLookup });
}
