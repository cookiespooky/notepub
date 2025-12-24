"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { useNav } from "./NavContext";

export function Reveal({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { isNavigating } = useNav();
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isFading, setIsFading] = useState(false);

  useEffect(() => {
    if (isNavigating) {
      setShowSkeleton(true);
      setIsFading(true);
    }
  }, [isNavigating]);

  useEffect(() => {
    setIsFading(true);
    const fadeTimer = setTimeout(() => setIsFading(false), 160);
    const hideTimer = setTimeout(() => setShowSkeleton(false), 260);
    return () => {
      clearTimeout(fadeTimer);
      clearTimeout(hideTimer);
    };
  }, [pathname]);

  const classNames = ["np-reveal"];
  if (showSkeleton) classNames.push("np-reveal-loading");
  if (isFading) classNames.push("np-reveal-fading");

  return (
    <div className={classNames.join(" ")}>
      <div className="np-reveal-skeleton" aria-hidden="true" />
      <div className="np-reveal-content">{children}</div>
    </div>
  );
}
