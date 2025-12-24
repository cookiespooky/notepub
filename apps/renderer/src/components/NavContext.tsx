"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import Link, { type LinkProps } from "next/link";
import { usePathname, useRouter } from "next/navigation";

type NavContextValue = {
  isNavigating: boolean;
  startNavigation: (href: string) => void;
};

const NavContext = createContext<NavContextValue | undefined>(undefined);

export function NavProvider({ children }: { children: React.ReactNode }) {
  const [isNavigating, setIsNavigating] = useState(false);
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    // Clear navigating state when path changes (navigation settled)
    setIsNavigating(false);
  }, [pathname]);

  const value = useMemo<NavContextValue>(
    () => ({
      isNavigating,
      startNavigation: (href: string) => {
        setIsNavigating(true);
        router.push(href);
      },
    }),
    [isNavigating, router],
  );

  return <NavContext.Provider value={value}>{children}</NavContext.Provider>;
}

export function useNav() {
  const ctx = useContext(NavContext);
  if (!ctx) throw new Error("useNav must be used within NavProvider");
  return ctx;
}

type NavLinkProps = LinkProps &
  Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> & {
    children: React.ReactNode;
  };

export function NavLink({ href, children, onClick, ...rest }: NavLinkProps) {
  const { startNavigation } = useNav();
  const currentPath = usePathname();
  const isExternal = typeof href === "string" ? /^https?:\/\//i.test(href) || href.startsWith("//") : false;
  const targetAttr = rest.target ?? (isExternal ? "_blank" : undefined);
  const relAttr = rest.rel ?? (isExternal ? "noreferrer" : undefined);

  const normalize = (input: string) => {
    if (!input) return "/";
    return input.endsWith("/") && input !== "/" ? input.slice(0, -1) : input;
  };

  const handleClick: React.MouseEventHandler<HTMLAnchorElement> = (e) => {
    if (onClick) onClick(e);
    if (
      isExternal ||
      e.defaultPrevented ||
      e.metaKey ||
      e.ctrlKey ||
      e.shiftKey ||
      e.altKey ||
      (rest.target && rest.target !== "_self")
    ) {
      return;
    }
    const target = typeof href === "string" ? href : ("pathname" in href ? href.pathname || "" : "");
    const normalizedTarget = normalize(target || (typeof href === "string" ? href : ""));
    const normalizedCurrent = normalize(currentPath || "");
    if (normalizedTarget === normalizedCurrent) {
      // Same page; let default behavior proceed without navigation blur.
      return;
    }
    e.preventDefault();
    startNavigation(normalizedTarget);
  };

  return (
    <Link href={href} {...rest} target={targetAttr} rel={relAttr} onClick={handleClick}>
      {children}
    </Link>
  );
}
