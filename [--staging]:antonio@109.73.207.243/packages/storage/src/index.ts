import { DeleteObjectCommand, GetObjectCommand, HeadObjectCommand, ListObjectsV2Command, PutObjectCommand, S3Client } from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { loadEnv, normalizePrefix, resolveS3Prefix } from "@notepub/env";

const env = loadEnv();
const basePrefix = resolveS3Prefix(env);

const s3Client = new S3Client({
  region: env.S3_REGION || "ru-1",
  endpoint: env.S3_ENDPOINT || "https://s3.timeweb.com",
  credentials: {
    accessKeyId: env.S3_ACCESS_KEY,
    secretAccessKey: env.S3_SECRET_KEY,
  },
});

export type S3ObjectEntry = {
  key: string;
  etag: string;
  lastModified: string | null;
  size?: number;
};

export async function getObjectAsString(key: string, opts?: { ifNoneMatch?: string }) {
  const targetKey = prependBasePrefix(key);
  const response = await s3Client.send(
    new GetObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
      IfNoneMatch: opts?.ifNoneMatch,
    }),
  );

  const body = await streamToString(response.Body);
  return {
    status: 200,
    etag: response.ETag?.replace(/"/g, "") || "",
    lastModified: response.LastModified?.toISOString() || null,
    body,
  };
}

export async function getObjectBuffer(key: string) {
  const targetKey = prependBasePrefix(key);
  const response = await s3Client.send(
    new GetObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
    }),
  );
  const buffer = await streamToBuffer(response.Body);
  return {
    status: 200,
    etag: response.ETag?.replace(/"/g, "") || "",
    lastModified: response.LastModified?.toISOString() || null,
    contentType: response.ContentType || "application/octet-stream",
    contentLength: typeof response.ContentLength === "number" ? response.ContentLength : buffer.length,
    body: buffer,
  };
}

export async function listObjects(prefix: string): Promise<S3ObjectEntry[]> {
  const normalized = normalizePrefix(prefix);
  const effectivePrefix = mergePrefix(basePrefix, normalized);
  let continuationToken: string | undefined;
  const entries: S3ObjectEntry[] = [];

  do {
    const result = await s3Client.send(
      new ListObjectsV2Command({
        Bucket: env.S3_BUCKET,
        Prefix: effectivePrefix || undefined,
        ContinuationToken: continuationToken,
      }),
    );

    for (const object of result.Contents || []) {
      if (!object.Key || object.Key.endsWith("/")) continue;
      const keyWithoutBase = stripBasePrefix(object.Key);
      entries.push({
        key: keyWithoutBase,
        etag: object.ETag?.replace(/"/g, "") || "",
        lastModified: object.LastModified?.toISOString() || null,
        size: typeof object.Size === "number" ? object.Size : undefined,
      });
    }
    continuationToken = result.IsTruncated ? result.NextContinuationToken : undefined;
  } while (continuationToken);

  return entries;
}

export async function putObjectString(key: string, body: string, contentType = "text/plain") {
  const targetKey = prependBasePrefix(key);
  await s3Client.send(
    new PutObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
      Body: body,
      ContentType: contentType,
    }),
  );
}

export async function putObjectBuffer(key: string, body: Buffer | Uint8Array, contentType = "application/octet-stream") {
  const targetKey = prependBasePrefix(key);
  await s3Client.send(
    new PutObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
      Body: body,
      ContentType: contentType,
    }),
  );
}

export async function deleteObject(key: string) {
  const targetKey = prependBasePrefix(key);
  await s3Client.send(
    new DeleteObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
    }),
  );
}

export async function headObject(key: string) {
  const targetKey = prependBasePrefix(key);
  const response = await s3Client.send(
    new HeadObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
    }),
  );
  return {
    etag: response.ETag?.replace(/"/g, "") || "",
    lastModified: response.LastModified?.toISOString() || null,
    contentLength: typeof response.ContentLength === "number" ? response.ContentLength : 0,
    contentType: response.ContentType || "application/octet-stream",
  };
}

export async function getSignedObjectUrl(key: string, expiresIn = 3600) {
  const targetKey = prependBasePrefix(key);
  return getSignedUrl(
    s3Client,
    new GetObjectCommand({
      Bucket: env.S3_BUCKET,
      Key: targetKey,
    }),
    { expiresIn },
  );
}

function prependBasePrefix(key: string) {
  const normalizedKey = key.replace(/^\/+/, "");
  if (!basePrefix) return normalizedKey;
  if (normalizedKey.startsWith(basePrefix)) return normalizedKey;
  return `${basePrefix}${normalizedKey}`;
}

function mergePrefix(base: string, relative: string) {
  if (!base) return relative;
  if (!relative) return base;
  if (relative.startsWith(base)) return relative;
  return `${base}${relative}`;
}

function stripBasePrefix(key: string) {
  if (!basePrefix) return key.replace(/^\/+/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedKey.startsWith(basePrefix)) {
    return normalizedKey.slice(basePrefix.length);
  }
  return normalizedKey;
}

async function streamToString(body: unknown) {
  if (!body) return "";
  if (typeof body === "string") return body;
  if (Buffer.isBuffer(body)) return body.toString("utf8");
  const chunks: Uint8Array[] = [];
  if (isAsyncIterable(body)) {
    for await (const chunk of body) {
      chunks.push(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
    }
    return Buffer.concat(chunks).toString("utf8");
  }
  return "";
}

async function streamToBuffer(body: unknown): Promise<Buffer> {
  if (!body) return Buffer.alloc(0);
  if (Buffer.isBuffer(body)) return body;
  const chunks: Uint8Array[] = [];
  if (isAsyncIterable(body)) {
    for await (const chunk of body) {
      chunks.push(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
    }
    return Buffer.concat(chunks);
  }
  return Buffer.alloc(0);
}

function isAsyncIterable(value: unknown): value is AsyncIterable<Uint8Array | string> {
  return typeof value === "object" && value !== null && Symbol.asyncIterator in value;
}
