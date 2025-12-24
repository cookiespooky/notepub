import type React from "react";
import { headers } from "next/headers";
import { getSiteBySlug } from "@notepub/core";
import { getThemeSettings, themeToCssVars } from "@/lib/theme";

type EditorLayoutProps = {
  children: React.ReactNode;
};

export default async function EditorLayout({ children }: EditorLayoutProps) {
  const hostHeader = headers().get("host") || "";
  const siteSlug = headers().get("x-site-slug") || extractSlugFromHost(hostHeader);
  const site = siteSlug ? await getSiteBySlug(siteSlug) : null;
  const theme =
    site && typeof (site as any).s3Prefix === "string"
      ? await getThemeSettings((site as any).s3Prefix)
      : null;
  const themeVars = theme ? themeToCssVars(theme) : "";
  const themeJson = theme ? JSON.stringify(theme) : "";

  return (
    <>
      {themeVars ? (
        <style
          dangerouslySetInnerHTML={{
            __html: `:root{${themeVars}}`,
          }}
        />
      ) : null}
      {themeJson ? (
        <script
          // eslint-disable-next-line react/no-danger
          dangerouslySetInnerHTML={{
            __html: `window.__EDITOR_THEME=${themeJson};`,
          }}
        />
      ) : null}
      {children}
    </>
  );
}

function extractSlugFromHost(host: string) {
  const withoutPort = host.split(":")[0] || "";
  const parts = withoutPort.split(".").filter(Boolean);
  if (parts.length === 2 && parts[1] === "localhost") {
    return parts[0];
  }
  if (parts.length <= 2) return "";
  return parts[0];
}
