import fs from "fs";
import path from "path";
import { cacheRoot, notesCacheDir } from "./config";

async function ensureDir(dir: string) {
  await fs.promises.mkdir(dir, { recursive: true });
}

export async function readJSON<T>(filePath: string): Promise<T | null> {
  try {
    const raw = await fs.promises.readFile(filePath, "utf8");
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

export async function writeJSON(filePath: string, data: unknown) {
  await ensureDir(path.dirname(filePath));
  await fs.promises.writeFile(filePath, JSON.stringify(data, null, 2), "utf8");
}

export function getIndexCachePath() {
  return path.join(cacheRoot, "index.json");
}

export function getNoteCachePath(objectKey: string) {
  const safeName = Buffer.from(objectKey).toString("hex");
  return path.join(notesCacheDir, `${safeName}.json`);
}

export function getFolderCachePath(folderPath: string) {
  const safeName = Buffer.from(folderPath).toString("hex");
  return path.join(cacheRoot, "folders", `${safeName}.json`);
}
