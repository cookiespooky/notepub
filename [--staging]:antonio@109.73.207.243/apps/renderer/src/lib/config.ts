import path from "path";
import { loadEnv, resolveS3Prefix } from "@notepub/env";

export const cacheRoot = path.join(process.cwd(), "cache");
export const notesCacheDir = path.join(cacheRoot, "notes");
export const folderCacheDir = path.join(cacheRoot, "folders");

export function getS3Config(): S3Config {
  const env = loadEnv();
  const endpoint = env.S3_ENDPOINT || "https://s3.timeweb.com";
  const region = env.S3_REGION || "ru-1";
  const bucket = env.S3_BUCKET || process.env.S3_BUCKET_NAME || "";
  const accessKeyId = env.S3_ACCESS_KEY || "";
  const secretAccessKey = env.S3_SECRET_KEY || "";
  const prefix = resolveS3Prefix(env);

  if (!bucket) {
    throw new Error("S3_BUCKET env var is required");
  }
  if (!accessKeyId || !secretAccessKey) {
    throw new Error("S3_ACCESS_KEY and S3_SECRET_KEY env vars are required");
  }

  return { endpoint, region, bucket, accessKeyId, secretAccessKey, prefix };
}

export type S3Config = {
  endpoint: string;
  region: string;
  bucket: string;
  accessKeyId: string;
  secretAccessKey: string;
  prefix: string;
};
