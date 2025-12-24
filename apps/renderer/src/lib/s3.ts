import {
  CopyObjectCommand,
  DeleteObjectCommand,
  GetObjectCommand,
  HeadObjectCommand,
  ListObjectsV2Command,
  PutObjectCommand,
  S3Client,
} from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { getS3Config } from "./config";
import { S3ObjectEntry } from "./types";

let client: S3Client | null = null;

export function getClient() {
  if (!client) {
    const config = getS3Config();
    client = new S3Client({
      region: config.region,
      endpoint: config.endpoint,
      credentials: {
        accessKeyId: config.accessKeyId,
        secretAccessKey: config.secretAccessKey,
      },
    });
  }
  return client;
}

export async function listNoteObjects(customPrefix?: string): Promise<S3ObjectEntry[]> {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const effectivePrefix = buildPrefix(basePrefix, customPrefix);
  let continuationToken: string | undefined;
  const entries: S3ObjectEntry[] = [];

  do {
    const result = await s3.send(
      new ListObjectsV2Command({
        Bucket: bucket,
        Prefix: effectivePrefix || undefined,
        ContinuationToken: continuationToken,
      }),
    );

    for (const object of result.Contents || []) {
      if (!object.Key || object.Key.endsWith("/")) continue;
      if (!object.Key.toLowerCase().endsWith(".md")) continue;
      const relativeKey = stripBasePrefix(object.Key, basePrefix);
      entries.push({
        key: relativeKey,
        etag: object.ETag?.replace(/"/g, "") || "",
        lastModified: object.LastModified?.toISOString() || null,
        size: typeof object.Size === "number" ? object.Size : undefined,
      });
    }
    continuationToken = result.IsTruncated ? result.NextContinuationToken : undefined;
  } while (continuationToken);

  return entries;
}

export async function listFolderPlaceholders(customPrefix?: string): Promise<S3ObjectEntry[]> {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const effectivePrefix = buildPrefix(basePrefix, customPrefix);
  let continuationToken: string | undefined;
  const entries: S3ObjectEntry[] = [];

  do {
    const result = await s3.send(
      new ListObjectsV2Command({
        Bucket: bucket,
        Prefix: effectivePrefix || undefined,
        ContinuationToken: continuationToken,
      }),
    );

    for (const object of result.Contents || []) {
      if (!object.Key || object.Key.endsWith("/")) continue;
      if (!object.Key.toLowerCase().endsWith("/.keep")) continue;
      const relativeKey = stripBasePrefix(object.Key, effectivePrefix);
      entries.push({
        key: relativeKey,
        etag: object.ETag?.replace(/"/g, "") || "",
        lastModified: object.LastModified?.toISOString() || null,
        size: typeof object.Size === "number" ? object.Size : undefined,
      });
    }
    continuationToken = result.IsTruncated ? result.NextContinuationToken : undefined;
  } while (continuationToken);

  return entries;
}

export async function putObject(key: string, body: string, customPrefix?: string) {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const targetKey = buildTargetKey(key, basePrefix, customPrefix);
  await s3.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: targetKey,
      Body: body,
    }),
  );
  return { key: targetKey };
}

export async function putBinaryObject(key: string, body: Buffer, customPrefix?: string, contentType?: string) {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const targetKey = buildTargetKey(key, basePrefix, customPrefix);
  await s3.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: targetKey,
      Body: body,
      ContentType: contentType,
    }),
  );
  return { key: targetKey };
}

export async function deleteObject(key: string, customPrefix?: string) {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const targetKey = buildTargetKey(key, basePrefix, customPrefix);
  await s3.send(
    new DeleteObjectCommand({
      Bucket: bucket,
      Key: targetKey,
    }),
  );
  return { key: targetKey };
}

export async function copyObject(fromKey: string, toKey: string, customPrefix?: string) {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const sourceKey = buildTargetKey(fromKey, basePrefix, customPrefix);
  const targetKey = buildTargetKey(toKey, basePrefix, customPrefix);
  const encodedSource = encodeURIComponent(`/${bucket}/${sourceKey}`);
  await s3.send(
    new CopyObjectCommand({
      Bucket: bucket,
      CopySource: encodedSource,
      Key: targetKey,
    }),
  );
  return { key: targetKey };
}

function normalizePrefix(input: string | null | undefined) {
  if (!input) return "";
  let p = input.replace(/^\/+/, "");
  if (p && !p.endsWith("/")) p = `${p}/`;
  return p;
}

export async function headObject(key: string) {
  const s3 = getClient();
  const { bucket, prefix } = getS3Config();
  const targetKey = prependBasePrefix(key, prefix);
  const response = await s3.send(
    new HeadObjectCommand({
      Bucket: bucket,
      Key: targetKey,
    }),
  );
  return response;
}

export async function fetchObject(key: string, ifNoneMatch?: string) {
  const s3 = getClient();
  const { bucket, prefix } = getS3Config();
  const targetKey = prependBasePrefix(key, prefix);
  try {
    const response = await s3.send(
      new GetObjectCommand({
        Bucket: bucket,
        Key: targetKey,
        IfNoneMatch: ifNoneMatch,
      }),
    );
    const body = await streamToString(response.Body);
    return {
      status: 200,
      etag: response.ETag?.replace(/"/g, "") || "",
      lastModified: response.LastModified?.toISOString() || null,
      body,
    };
  } catch (error: unknown) {
    // S3 returns NotModified on 304
    if (isNotModified(error)) {
      return { status: 304, etag: ifNoneMatch?.replace(/"/g, "") || "", body: "" };
    }
    throw error instanceof Error ? error : new Error("Failed to fetch object");
  }
}

export async function getSignedObjectUrl(key: string, expiresIn = 3600) {
  const s3 = getClient();
  const { bucket, prefix } = getS3Config();
  const targetKey = prependBasePrefix(key, prefix);
  return getSignedUrl(
    s3,
    new GetObjectCommand({
      Bucket: bucket,
      Key: targetKey,
    }),
    { expiresIn },
  );
}

export async function getSignedObjectUrlWithPrefix(key: string, customPrefix?: string, expiresIn = 3600) {
  const s3 = getClient();
  const { bucket, prefix: basePrefix } = getS3Config();
  const targetKey = buildTargetKey(key, basePrefix, customPrefix);
  return getSignedUrl(
    s3,
    new GetObjectCommand({
      Bucket: bucket,
      Key: targetKey,
    }),
    { expiresIn },
  );
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

function isAsyncIterable(value: unknown): value is AsyncIterable<Uint8Array | string> {
  return typeof value === "object" && value !== null && Symbol.asyncIterator in value;
}

function isNotModified(error: unknown) {
  if (!error || typeof error !== "object") return false;
  const meta = (error as { $metadata?: { httpStatusCode?: number } }).$metadata;
  return meta?.httpStatusCode === 304;
}

function buildPrefix(basePrefix: string, customPrefix?: string | null) {
  const normalizedCustom = normalizePrefix(customPrefix);
  if (!normalizedCustom) return basePrefix;
  if (!basePrefix) return normalizedCustom;
  if (normalizedCustom.startsWith(basePrefix)) return normalizedCustom;
  return `${basePrefix}${normalizedCustom}`;
}

function prependBasePrefix(key: string, basePrefix: string) {
  const normalizedKey = key.replace(/^\/+/, "");
  if (!basePrefix) return normalizedKey;
  if (normalizedKey.startsWith(basePrefix)) return normalizedKey;
  return `${basePrefix}${normalizedKey}`;
}

function stripBasePrefix(key: string, basePrefix: string) {
  if (!basePrefix) return key.replace(/^\/+/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedKey.startsWith(basePrefix)) {
    return normalizedKey.slice(basePrefix.length);
  }
  return normalizedKey;
}

function buildTargetKey(key: string, basePrefix: string, customPrefix?: string) {
  const normalizedKey = key.replace(/^\/+/, "");
  const effectivePrefix = buildPrefix(basePrefix, customPrefix);
  if (!effectivePrefix) return normalizedKey;
  return `${effectivePrefix}${normalizedKey}`;
}
