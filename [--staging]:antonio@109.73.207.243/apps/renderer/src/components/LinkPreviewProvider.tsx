"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import styles from "./LinkPreviewProvider.module.css";

type Preview = { title: string; preview: string };
type PreviewMap = Record<string, Preview>;

type ActivePreview = Preview & {
  slug: string;
  x: number;
  y: number;
};

type Props = {
  previews: PreviewMap;
  children: ReactNode;
};

export function LinkPreviewProvider({ previews, children }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const hideTimer = useRef<NodeJS.Timeout | null>(null);
  const [active, setActive] = useState<ActivePreview | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const links = Array.from(container.querySelectorAll<HTMLAnchorElement>("a[data-note-slug]"));

    const handleEnter = (event: Event) => {
      const target = event.currentTarget as HTMLElement | null;
      if (!target) return;
      const slug = target.getAttribute("data-note-slug");
      if (!slug) return;
      const preview = previews[slug];
      if (!preview) return;
      if (hideTimer.current) {
        clearTimeout(hideTimer.current);
        hideTimer.current = null;
      }
      const rect = target.getBoundingClientRect();
      setActive({
        slug,
        title: preview.title,
        preview: preview.preview,
        x: rect.left + rect.width / 2,
        y: rect.bottom + 8,
      });
    };

    const handleLeave = () => {
      if (hideTimer.current) clearTimeout(hideTimer.current);
      hideTimer.current = setTimeout(() => setActive(null), 120);
    };

    links.forEach((link) => {
      link.addEventListener("mouseenter", handleEnter);
      link.addEventListener("mouseleave", handleLeave);
      link.addEventListener("focus", handleEnter);
      link.addEventListener("blur", handleLeave);
    });

    return () => {
      links.forEach((link) => {
        link.removeEventListener("mouseenter", handleEnter);
        link.removeEventListener("mouseleave", handleLeave);
        link.removeEventListener("focus", handleEnter);
        link.removeEventListener("blur", handleLeave);
      });
      if (hideTimer.current) {
        clearTimeout(hideTimer.current);
        hideTimer.current = null;
      }
    };
  }, [previews]);

  return (
    <div ref={containerRef} className={styles.previewContainer}>
      {children}
      {active && (
        <div className={styles.preview} style={{ top: active.y, left: active.x }}>
          <div className={styles.title}>{active.title}</div>
          {active.preview && <div className={styles.snippet}>{active.preview}</div>}
        </div>
      )}
    </div>
  );
}
