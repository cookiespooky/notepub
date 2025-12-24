import type { Metadata } from "next";
import "./globals.css";
import { getAppUrl } from "../lib/domains";
import { NavProvider } from "@/components/NavContext";
import { SidebarDataProvider } from "@/components/SidebarDataContext";

const appUrl = getAppUrl();
export const metadata: Metadata = {
  title: { default: "Notepub", template: "%s | Notepub" },
  description: "Live site rendered with Notepub",
  metadataBase: new URL(appUrl),
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <SidebarDataProvider>
          <NavProvider>{children}</NavProvider>
        </SidebarDataProvider>
      </body>
    </html>
  );
}
