import type { Metadata } from "next";
import React from "react";
import "./globals.css";
import { getAppUrl } from "@/lib/domains";

const appUrl = getAppUrl();
export const metadata: Metadata = {
  title: "Notepub",
  description: "Build sites from your Obsidian vault",
  openGraph: {
    title: "Notepub",
    description: "Build sites from your Obsidian vault",
    url: appUrl,
    siteName: "Notepub",
    images: [{ url: `${appUrl}/og-default.svg`, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Notepub",
    description: "Build sites from your Obsidian vault",
    images: [`${appUrl}/og-default.svg`],
  },
  icons: {
    icon: "/favicon.ico",
    apple: "/apple-touch-icon.png",
  },
  metadataBase: new URL(appUrl),
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
