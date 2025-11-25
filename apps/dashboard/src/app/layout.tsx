import type { Metadata } from "next";
import React from "react";
import "./globals.css";

export const metadata: Metadata = {
  title: "Notepub",
  description: "Build sites from your Obsidian vault",
  openGraph: {
    title: "Notepub",
    description: "Build sites from your Obsidian vault",
    url: "https://notepub.site",
    siteName: "Notepub",
    images: [{ url: "/og-default.svg", width: 1200, height: 630 }],
  },
  icons: {
    icon: "/favicon.ico",
    apple: "/apple-touch-icon.png",
  },
  metadataBase: new URL("https://notepub.site"),
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
