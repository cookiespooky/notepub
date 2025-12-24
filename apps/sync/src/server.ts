import express, { type Request, type Response } from "express";
import basicAuth from "basic-auth";
import { loadEnv } from "@notepub/env";
import { findUserByToken } from "@notepub/core";
import { prisma } from "@notepub/db";
import { deleteObject, getObjectBuffer, headObject, listObjects, putObjectBuffer } from "@notepub/storage";

const env = loadEnv();
const app = express();
const port = Number(process.env.SYNC_PORT || (env as any).SYNC_PORT || (env as any).PORT || 3201);

app.use(express.raw({ type: "*/*", limit: "100mb" }));

app.get("/healthz", (_req: Request, res: Response) => {
  res.json({ ok: true });
});

app.all("/webdav/*", async (req: Request, res: Response) => {
  const creds = basicAuth(req);
  if (!creds || !creds.name || !creds.pass) {
    res.setHeader("WWW-Authenticate", "Basic realm=\"Notepub WebDAV\"");
    return res.status(401).end();
  }

  const user = await findUserByToken(creds.name, creds.pass);
  if (!user) {
    res.setHeader("WWW-Authenticate", "Basic realm=\"Notepub WebDAV\"");
    return res.status(401).end();
  }

  const rawPath = req.path.replace(/^\/webdav\/?/, "") || "";
  const decodedPath = safeDecodePath(rawPath);
  const webdavPath = decodedPath.replace(/^\/+/, "");
  const { relativePath, displayBase } = normalizeWebdavPath(webdavPath, creds.name);
  const vaultBase = `publishers/${user.id}/vaults/${user.siteId}`;
  const key = relativePath ? `${vaultBase}/${relativePath}` : vaultBase;

  switch (req.method) {
    case "OPTIONS": {
      res.setHeader("Allow", "OPTIONS, PROPFIND, GET, PUT, DELETE, MKCOL, HEAD");
      res.setHeader("DAV", "1,2");
      res.setHeader("MS-Author-Via", "DAV");
      return res.status(204).end();
    }
    case "HEAD": {
      try {
        const meta = await headObject(key);
        res.setHeader("Content-Length", String(meta.contentLength || 0));
        res.setHeader("Content-Type", meta.contentType || "application/octet-stream");
        return res.status(200).end();
      } catch (error) {
        return res.status(404).end();
      }
    }
    case "GET": {
      try {
        const obj = await getObjectBuffer(key);
        res.setHeader("Content-Type", obj.contentType || "application/octet-stream");
        res.setHeader("Content-Length", String(obj.contentLength));
        return res.status(200).send(obj.body);
      } catch (error) {
        return res.status(404).send("Not Found");
      }
    }
    case "PUT": {
      const body = Buffer.isBuffer(req.body) ? (req.body as Buffer) : Buffer.from(req.body || "");
      const contentType = req.headers["content-type"] || "application/octet-stream";
      const incomingSize = BigInt(body.length);
      const existingSize = await getSizeOrZero(key);
      const delta = incomingSize - existingSize;
      const quota = BigInt(user.quotaBytes ?? BigInt(10 * 1024 * 1024));
      const used = BigInt(user.usedBytes ?? BigInt(0));

      if (delta > 0n && used + delta > quota) {
        res.status(507).send("Storage quota exceeded. Please delete files or upgrade your plan.");
        return;
      }

      try {
        await putObjectBuffer(key, body, Array.isArray(contentType) ? contentType[0] : contentType);
        await bumpUsage(user.siteId, delta);
        return res.status(201).end();
      } catch (error) {
        return res.status(500).send("Upload failed");
      }
    }
    case "DELETE": {
      try {
        const size = await getSizeOrZero(key);
        await deleteObject(key);
        if (size > 0n) {
          await bumpUsage(user.siteId, -size);
        }
        return res.status(204).end();
      } catch (error) {
        return res.status(404).end();
      }
    }
    case "PROPFIND": {
      const depth = req.headers["depth"] ?? "1";
      const isDir = req.path.endsWith("/") || webdavPath.endsWith("/");
      if (isDir) {
        const prefixKey = key.endsWith("/") ? key : `${key}/`;
        const entries = await listObjects(prefixKey);
        const body = renderPropfind(prefixKey, entries, depth === "0", displayBase, relativePath);
        res.setHeader("Content-Type", "application/xml; charset=utf-8");
        return res.status(207).send(body);
      }
      try {
        const meta = await headObject(key);
        const body = renderSinglePropfind(key, meta.contentLength, meta.lastModified || new Date().toISOString(), displayBase, relativePath);
        res.setHeader("Content-Type", "application/xml; charset=utf-8");
        return res.status(207).send(body);
      } catch (error) {
        return res.status(404).end();
      }
    }
    case "MKCOL": {
      return res.status(201).end();
    }
    default:
      return res.status(405).end();
  }
});

app.listen(port, () => {
  console.log(`Sync service listening on ${port}`);
});

function renderPropfind(
  prefix: string,
  entries: { key: string; size?: number; lastModified: string | null }[],
  shallowOnly: boolean,
  displayBase: string,
  relBase: string,
) {
  const relBaseNorm = relBase.replace(/^\/+|\/+$/g, "");
  const hrefPrefix = ensureTrailingSlash(displayBase + relBaseNorm);
  const folders = new Set<string>();
  const files: { href: string; size: number; modified: string }[] = [];
  for (const entry of entries) {
    const rel = stripPrefix(entry.key, prefix);
    if (!rel) continue;
    const parts = rel.split("/").filter(Boolean);
    if (parts.length > 1 && shallowOnly) {
      folders.add(parts[0]);
      continue;
    }
    if (parts.length > 1) {
      folders.add(parts[0]);
      continue;
    }
    files.push({ href: hrefPrefix + parts[0], size: entry.size || 0, modified: entry.lastModified || new Date().toISOString() });
  }

  const folderXml = [...folders]
    .map((name) => renderCollection(hrefPrefix + ensureTrailingSlash(name)))
    .join("");
  const fileXml = files.map((f) => renderResource(f.href, f.size, f.modified)).join("");

  return wrapMultiStatus(folderXml + fileXml || renderCollection(hrefPrefix));
}

function renderSinglePropfind(hrefKey: string, size: number, modified: string, displayBase: string, relPath: string) {
  const relNorm = relPath.replace(/^\/+/, "");
  const href = relNorm ? `${displayBase}${relNorm}` : ensureTrailingSlash(displayBase);
  return wrapMultiStatus(renderResource(href, size, modified));
}

function renderCollection(href: string) {
  return `<response><href>/${escapeXml(href)}</href><propstat><prop><resourcetype><collection/></resourcetype></prop><status>HTTP/1.1 200 OK</status></propstat></response>`;
}

function renderResource(href: string, size: number, modified: string) {
  return `<response><href>/${escapeXml(href)}</href><propstat><prop><getcontentlength>${size}</getcontentlength><getlastmodified>${modified}</getlastmodified><resourcetype/></prop><status>HTTP/1.1 200 OK</status></propstat></response>`;
}

function wrapMultiStatus(body: string) {
  return `<?xml version="1.0" encoding="utf-8"?><multistatus xmlns="DAV:">${body}</multistatus>`;
}

function stripPrefix(key: string, prefix: string) {
  const normalized = prefix.replace(/^\/+/, "");
  const normalizedKey = key.replace(/^\/+/, "");
  if (normalizedKey.startsWith(normalized)) {
    return normalizedKey.slice(normalized.length).replace(/^\/+/, "");
  }
  return normalizedKey;
}

async function getSizeOrZero(key: string): Promise<bigint> {
  try {
    const meta = await headObject(key);
    return BigInt(meta.contentLength || 0);
  } catch {
    return 0n;
  }
}

const REQUIRED_BASE = "notepub";

function normalizeWebdavPath(path: string, username: string) {
  const cleaned = safeDecodePath(path).replace(/^\/+/, "");
  const parts = cleaned.split("/").filter(Boolean);
  const userLocal = username.split("@")[0] || username;

  let remaining = parts;
  let hasUserPrefix = false;

  if (remaining[0] === username || remaining[0] === userLocal) {
    remaining = remaining.slice(1);
    hasUserPrefix = true;
  }

  if (remaining[0] === REQUIRED_BASE) {
    remaining = remaining.slice(1);
  }

  const relativePath = remaining.join("/");
  const displayBase = hasUserPrefix ? `/webdav/${userLocal}/${REQUIRED_BASE}/` : `/webdav/${REQUIRED_BASE}/`;

  return { relativePath, displayBase };
}

function safeDecodePath(input: string) {
  try {
    return decodeURIComponent(input);
  } catch {
    return input;
  }
}

function escapeXml(str: string) {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function ensureTrailingSlash(input: string) {
  if (!input) return "/";
  return input.endsWith("/") ? input : `${input}/`;
}

async function bumpUsage(siteId: string, delta: bigint) {
  if (delta === 0n) return;
  if (delta > 0n) {
    await prisma.site.update({
      where: { id: siteId },
      data: { vaultUsedBytes: { increment: delta as unknown as bigint } } as any,
    });
  } else {
    await prisma.site.update({
      where: { id: siteId },
      data: { vaultUsedBytes: { decrement: (delta * -1n) as unknown as bigint } } as any,
    });
  }
}
