"use client";

import { createPortal } from "react-dom";
import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
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
  const previewRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const ensureProse = () => {
      const article = container.querySelector("article");
      if (article && !article.classList.contains("prose")) {
        article.classList.add("prose");
      }
    };
    ensureProse();

    const observer = new MutationObserver(() => ensureProse());
    observer.observe(container, { childList: true, subtree: true });

    return () => observer.disconnect();
  }, []);

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
      const viewportWidth = window.innerWidth;
      const xCenter = rect.left + rect.width / 2;
      const yBottom = rect.bottom + 8;
      const clampedX = Math.min(Math.max(xCenter, 12), viewportWidth - 12);
      setActive({
        slug,
        title: preview.title,
        preview: preview.preview,
        x: clampedX,
        y: yBottom,
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

  useLayoutEffect(() => {
    if (!active) {
      setPosition(null);
      return;
    }
    const updatePosition = () => {
      const tooltip = previewRef.current;
      if (!tooltip) return;
      const { width } = tooltip.getBoundingClientRect();
      const padding = 12;
      const vw = window.innerWidth;
      const left = Math.min(Math.max(active.x - width / 2, padding), vw - padding - width);
      setPosition({ left, top: active.y });
    };
    updatePosition();
    window.addEventListener("resize", updatePosition);
    return () => window.removeEventListener("resize", updatePosition);
  }, [active]);

  return (
    <div ref={containerRef} className={styles.previewContainer}>
      {children}
      {active &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            ref={previewRef}
            className={styles.preview}
            style={{ top: position?.top ?? active.y, left: position?.left ?? active.x }}
          >
            <div className={styles.title}>{active.title}</div>
            {active.preview && <div className={styles.snippet}>{active.preview}</div>}
          </div>,
          document.body,
        )}
    </div>
  );
}
