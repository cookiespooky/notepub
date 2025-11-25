import { GetObjectCommand, S3Client } from "@aws-sdk/client-s3";
import { loadEnv, resolveS3Prefix } from "@notepub/env";
import { getSiteBySlug } from "@notepub/core";
import { headers } from "next/headers";
import fs from "fs/promises";
import path from "path";

const env = loadEnv();
const basePrefix = resolveS3Prefix(env);

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

  if (slug) {
    const site = await getSiteBySlug(slug);
    if (site?.s3Prefix) {
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

function prependBasePrefix(key: string) {
  const normalizedKey = key.replace(/^\/+/, "");
  if (!basePrefix) return normalizedKey;
  if (normalizedKey.startsWith(basePrefix)) return normalizedKey;
  return `${basePrefix}${normalizedKey}`;
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
    return new Response(body, {
      status: 200,
      headers: {
        "Content-Type": res.ContentType || contentType,
        "Cache-Control": "public, max-age=3600",
      },
    });
  } catch (error) {
    console.warn("favicon not found in S3", key, error);
    return null;
  }
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
