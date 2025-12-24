import { GetObjectCommand, S3Client } from "@aws-sdk/client-s3";
import { loadEnv, resolveS3Prefix } from "@notepub/env";
import { getSiteBySlug } from "@notepub/core";
import { headers } from "next/headers";
import fs from "fs/promises";
import path from "path";

const env = loadEnv();
const basePrefix = resolveS3Prefix(env);

type SiteWithOg = NonNullable<Awaited<ReturnType<typeof getSiteBySlug>>> & {
  ogImageUrl?: string | null;
};

const s3 = new S3Client({
  region: env.S3_REGION || "ru-1",
  endpoint: env.S3_ENDPOINT || "https://s3.timeweb.com",
  credentials: {
    accessKeyId: env.S3_ACCESS_KEY,
    secretAccessKey: env.S3_SECRET_KEY,
  },
});

export const size = 32;
export const contentType = "image/x-icon";

export default async function Icon() {
  const host = headers().get("host") || "";
  const slug = extractSubdomain(host);
  const site = slug ? await getSiteBySlug(slug) : null;

  if (site) {
    const fromOgImage = await tryGetFromOgImage(site as SiteWithOg);
    if (fromOgImage) return fromOgImage;

    if (site.s3Prefix) {
      const key = `${normalizePrefix(site.s3Prefix)}favicon.ico`;
      const fromS3 = await tryGetFromS3(key);
      if (fromS3) return fromS3;
    }
  }

  return defaultFavicon();
}

function extractSubdomain(host: string) {
  const withoutPort = host.split(":")[0] || "";
  const parts = withoutPort.split(".").filter(Boolean);
  if (parts.length === 2 && parts[1] === "localhost") {
    return parts[0];
  }
  if (parts.length <= 2) return "";
  return parts[0];
}

function normalizePrefix(prefix: string) {
  const trimmed = prefix.replace(/^\/+/, "").replace(/\/+$/, "");
  return trimmed ? `${trimmed}/` : "";
}

function normalizePath(pathname: string) {
  return pathname.replace(/^\/+/, "");
}

function prependBasePrefix(key: string) {
  const normalizedKey = key.replace(/^\/+/, "");
  if (!basePrefix) return normalizedKey;
  if (normalizedKey.startsWith(basePrefix)) return normalizedKey;
  return `${basePrefix}${normalizedKey}`;
}

async function tryGetFromOgImage(site: SiteWithOg) {
  const og = site?.ogImageUrl?.trim();
  if (!og) return null;

  if (og.startsWith("http://") || og.startsWith("https://")) {
    return tryFetchRemote(og);
  }

  const sitePath = normalizePath(og);
  const key = site.s3Prefix ? `${normalizePrefix(site.s3Prefix)}${sitePath}` : sitePath;
  if (!key) return null;
  return tryGetFromS3(key);
}

async function tryGetFromS3(key: string) {
  try {
    const res = await s3.send(
      new GetObjectCommand({
        Bucket: env.S3_BUCKET,
        Key: prependBasePrefix(key),
      }),
    );
    const body = await streamToBuffer(res.Body);
    if (!body) return null;
    return toFaviconResponse(body, res.ContentType);
  } catch (error) {
    if (!isNotFound(error)) {
      console.warn("favicon fetch error from S3", key, error);
    }
    return null;
  }
}

async function tryFetchRemote(url: string) {
  try {
    const res = await fetch(url);
    if (!res.ok) return null;
    const arrayBuffer = await res.arrayBuffer();
    const buffer = Buffer.from(arrayBuffer);
    return toFaviconResponse(buffer, res.headers.get("content-type") || undefined);
  } catch (error) {
    console.warn("favicon fetch failed", url, error);
    return null;
  }
}

function toFaviconResponse(buffer: Buffer, sourceContentType?: string | null) {
  // wrap the image into a square SVG to avoid distortion when the source is not square
  const mime = (sourceContentType || "image/png").split(";")[0];
  const b64 = buffer.toString("base64");
  // use slice to mimic object-fit: cover (centered, cropped to square)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}">` +
    `<rect width="${size}" height="${size}" fill="none"/>` +
    `<image href="data:${mime};base64,${b64}" width="${size}" height="${size}" preserveAspectRatio="xMidYMid slice"/>` +
    `</svg>`;

  return new Response(svg, {
    status: 200,
    headers: {
      "Content-Type": "image/svg+xml",
      "Cache-Control": "public, max-age=3600",
    },
  });
}

async function defaultFavicon() {
  const filePath = path.join(process.cwd(), "public", "favicon.ico");
  const file = await fs.readFile(filePath);
  return new Response(file, {
    status: 200,
    headers: {
      "Content-Type": contentType,
      "Cache-Control": "public, max-age=86400",
    },
  });
}

async function streamToBuffer(body: unknown) {
  if (!body) return null;
  if (body instanceof ReadableStream) {
    const reader = body.getReader();
    const chunks: Uint8Array[] = [];
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      if (value) chunks.push(value);
    }
    return Buffer.concat(chunks);
  }
  if (typeof (body as any).transformToByteArray === "function") {
    const arr = await (body as any).transformToByteArray();
    return Buffer.from(arr);
  }
  return null;
}

function isNotFound(error: unknown) {
  if (!error || typeof error !== "object") return false;
  const meta = (error as { $metadata?: { httpStatusCode?: number } }).$metadata;
  return meta?.httpStatusCode === 404;
}
