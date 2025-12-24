import type { Metadata } from "next";
import "./globals.css";
import { getAppUrl } from "../lib/domains";

const appUrl = getAppUrl();
export const metadata: Metadata = {
  title: "Obsidian Vault",
  description: "Live Obsidian vault rendered from Timeweb S3",
  metadataBase: new URL(appUrl),
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
