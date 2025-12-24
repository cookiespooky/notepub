import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

export function middleware(req: NextRequest) {
  const host = req.headers.get("host") || "";
  const subdomain = extractSubdomain(host);
  const requestHeaders = new Headers(req.headers);
  if (subdomain) {
    requestHeaders.set("x-site-slug", subdomain);
  }
  return NextResponse.next({
    request: {
      headers: requestHeaders,
    },
  });
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

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
