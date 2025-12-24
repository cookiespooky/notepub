import express from "express";
import basicAuth from "basic-auth";
import { loadEnv } from "@notepub/env";
import { findUserByToken } from "@notepub/core";
import { deleteObject, getObjectBuffer, headObject, listObjects, putObjectBuffer } from "@notepub/storage";
const env = loadEnv();
const app = express();
const port = Number(process.env.SYNC_PORT || env.SYNC_PORT || env.PORT || 3201);
app.use(express.raw({ type: "*/*", limit: "100mb" }));
app.get("/healthz", (_req, res) => {
    res.json({ ok: true });
});
app.all("/webdav/*", async (req, res) => {
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
    const webdavPath = (req.path.replace(/^\/webdav\/?/, "") || "").replace(/^\/+/, "");
    const key = `users/${user.id}/vault/${webdavPath}`;
    switch (req.method) {
        case "GET": {
            try {
                const obj = await getObjectBuffer(key);
                res.setHeader("Content-Type", obj.contentType || "application/octet-stream");
                res.setHeader("Content-Length", String(obj.contentLength));
                return res.status(200).send(obj.body);
            }
            catch (error) {
                return res.status(404).send("Not Found");
            }
        }
        case "PUT": {
            const body = Buffer.isBuffer(req.body) ? req.body : Buffer.from(req.body || "");
            const contentType = req.headers["content-type"] || "application/octet-stream";
            try {
                await putObjectBuffer(key, body, Array.isArray(contentType) ? contentType[0] : contentType);
                return res.status(201).end();
            }
            catch (error) {
                return res.status(500).send("Upload failed");
            }
        }
        case "DELETE": {
            try {
                await deleteObject(key);
                return res.status(204).end();
            }
            catch (error) {
                return res.status(404).end();
            }
        }
        case "PROPFIND": {
            const depth = req.headers["depth"] ?? "1";
            if (webdavPath.endsWith("/")) {
                const prefix = key.endsWith("/") ? key : `${key}/`;
                const entries = await listObjects(prefix);
                const body = renderPropfind(prefix, entries, depth === "0");
                res.setHeader("Content-Type", "application/xml; charset=utf-8");
                return res.status(207).send(body);
            }
            try {
                const meta = await headObject(key);
                const body = renderSinglePropfind(key, meta.contentLength, meta.lastModified || new Date().toISOString());
                res.setHeader("Content-Type", "application/xml; charset=utf-8");
                return res.status(207).send(body);
            }
            catch (error) {
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
function renderPropfind(prefix, entries, shallowOnly) {
    const hrefPrefix = ensureTrailingSlash(stripUserPrefix(prefix));
    const folders = new Set();
    const files = [];
    for (const entry of entries) {
        const rel = stripPrefix(entry.key, prefix);
        if (!rel)
            continue;
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
function renderSinglePropfind(href, size, modified) {
    return wrapMultiStatus(renderResource(stripUserPrefix(href), size, modified));
}
function renderCollection(href) {
    return `<response><href>/${escapeXml(href)}</href><propstat><prop><resourcetype><collection/></resourcetype></prop><status>HTTP/1.1 200 OK</status></propstat></response>`;
}
function renderResource(href, size, modified) {
    return `<response><href>/${escapeXml(href)}</href><propstat><prop><getcontentlength>${size}</getcontentlength><getlastmodified>${modified}</getlastmodified><resourcetype/></prop><status>HTTP/1.1 200 OK</status></propstat></response>`;
}
function wrapMultiStatus(body) {
    return `<?xml version="1.0" encoding="utf-8"?><multistatus xmlns="DAV:">${body}</multistatus>`;
}
function stripPrefix(key, prefix) {
    const normalized = prefix.replace(/^\/+/, "");
    const normalizedKey = key.replace(/^\/+/, "");
    if (normalizedKey.startsWith(normalized)) {
        return normalizedKey.slice(normalized.length).replace(/^\/+/, "");
    }
    return normalizedKey;
}
function escapeXml(str) {
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
function ensureTrailingSlash(input) {
    return input.endsWith("/") ? input : `${input}/`;
}
function stripUserPrefix(fullKey) {
    const withoutLeading = fullKey.replace(/^\/+/, "");
    const parts = withoutLeading.split("/");
    const rest = parts.slice(2).join("/");
    return rest;
}
