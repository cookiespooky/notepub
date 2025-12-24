import { NextRequest, NextResponse } from "next/server";
import { requireSiteFromRequest } from "@/lib/siteContext";
import { putBinaryObject } from "@/lib/s3";
import { createHash } from "crypto";
import path from "path";

export async function POST(req: NextRequest) {
  const context = await requireSiteFromRequest(req);
  if (!context) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const form = await req.formData().catch(() => null);
  if (!form) {
    return NextResponse.json({ error: "Invalid form data" }, { status: 400 });
  }

  const file = form.get("file");
  if (!(file instanceof File)) {
    return NextResponse.json({ error: "Missing file" }, { status: 400 });
  }

  const buffer = Buffer.from(await file.arrayBuffer());
  const ext = getExtension(file.type, file.name);
  const hash = createHash("sha1").update(buffer).digest("hex").slice(0, 10);
  const baseName = slugifyFilename(file.name.replace(/\.[^.]+$/, "")) || "image";
  const filename = `${baseName}-${hash}${ext ? `.${ext}` : ""}`;
  const folderRaw = (form.get("folder") || "").toString();
  const normalizedFolder = normalizeFolder(folderRaw);
  const key = normalizedFolder ? `${normalizedFolder}/${filename}` : filename;

  await putBinaryObject(key, buffer, context.prefix || undefined, file.type);

  // Return both the stored key and a relative path suitable for Obsidian-style links.
  return NextResponse.json({ url: `/api/editor/assets/${key}`, key });
}

function normalizeFolder(input: string) {
  const cleaned = input.trim().replace(/\\/g, "/");
  if (!cleaned) return "";
  const normalized = path.posix.normalize(cleaned).replace(/^\/+/, "");
  if (normalized === "." || normalized === "..") return "";
  if (normalized.startsWith("../")) return "";
  return normalized;
}

function slugifyFilename(name: string) {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9а-яё_-]+/gi, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/-{2,}/g, "-");
}

function getExtension(mime: string, fallbackName: string) {
  const map: Record<string, string> = {
    "image/png": "png",
    "image/jpeg": "jpg",
    "image/jpg": "jpg",
    "image/gif": "gif",
    "image/webp": "webp",
    "image/avif": "avif",
    "image/svg+xml": "svg",
  };
  const ext = map[mime];
  if (ext) return ext;
  const match = fallbackName.match(/\.([a-zA-Z0-9]+)$/);
  return match ? match[1].toLowerCase() : "";
}
