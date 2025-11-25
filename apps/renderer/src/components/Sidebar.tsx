"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import styles from "./Sidebar.module.css";
import type { IndexTreeNode, FlatNoteIndex } from "@/lib/types";

type SidebarProps = {
  tree: IndexTreeNode[];
  flat: Omit<FlatNoteIndex, "key">[];
  activeSlug: string;
  siteTitle?: string;
  siteAvatarUrl?: string | null;
};

export function Sidebar({ tree, flat, activeSlug, siteTitle, siteAvatarUrl }: SidebarProps) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [openSections, setOpenSections] = useState<Set<string>>(new Set());
  const brandName = siteTitle?.trim() || "Notepub";
  const avatarSrc = siteAvatarUrl?.trim() || "/logo.png";

  const activeNote = useMemo(() => flat.find((note) => note.slug === activeSlug), [flat, activeSlug]);
  const requiredOpenKeys = useMemo(() => {
    const paths = new Set<string>();
    if (!activeNote) return paths;
    const segments = activeNote.relativeKey.split("/").filter(Boolean);
    const folders = segments.slice(0, -1);
    for (let i = 1; i <= folders.length; i++) {
      paths.add(folders.slice(0, i).join("/"));
    }
    return paths;
  }, [activeNote]);
  const effectiveOpen = useMemo(() => {
    const merged = new Set(openSections);
    requiredOpenKeys.forEach((key) => merged.add(key));
    return merged;
  }, [openSections, requiredOpenKeys]);

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return [];
    return flat
      .filter((note) => note.title.toLowerCase().includes(q) || note.html.toLowerCase().includes(q))
      .slice(0, 30);
  }, [flat, query]);

  const closeMobile = () => setOpen(false);
  const toggleSection = (key: string) => {
    setOpenSections((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const rootNode = tree.find((n) => n.path.length === 0) || { title: "", path: [], children: [], folders: [] };

  const renderFolder = (node: IndexTreeNode) => {
    const key = node.path.join("/");
    const isOpen = effectiveOpen.has(key) || node.path.length === 0;
    return (
      <div key={key || "root"} className={styles.section}>
        {node.path.length > 0 && (
          <button className={styles.sectionHeader} type="button" onClick={() => toggleSection(key)} style={{ paddingLeft: 12 * node.path.length }}>
            <span className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ""}`}>▶</span>
            <span className={styles.sectionTitle}>{node.title || "Папка"}</span>
          </button>
        )}
        {isOpen && (
          <div>
            {node.children.map((child) => (
              <Link
                key={child.slug}
                href={`/${child.slug}`}
                onClick={closeMobile}
                className={`${styles.link} ${activeSlug === child.slug ? styles.active : ""}`}
                style={{ paddingLeft: 12 * (node.path.length + 1) }}
              >
                {child.title}
              </Link>
            ))}
            {node.folders?.map((folder) => renderFolder(folder))}
          </div>
        )}
      </div>
    );
  };

  return (
    <aside className={styles.sidebar}>
      <div className={styles.mobileHeader}>
        <button className={styles.mobileToggle} onClick={() => setOpen(true)} aria-label="Открыть меню">
          <span />
          <span />
          <span />
        </button>
        <div className={styles.mobileAvatar}>
          <div className={styles.avatar}>
            <img src={avatarSrc} alt={brandName} className={styles.avatarImage} />
          </div>
        </div>
      </div>
      <div className={`${styles.mobilePortal} ${open ? styles.visible : ""}`} onClick={closeMobile} />
      <div className={`${styles.panel} ${open ? styles.open : ""}`}>
        <div className={styles.brand}>
          <div className={styles.avatar}>
            <img src={avatarSrc} alt={brandName} className={styles.avatarImage} />
          </div>
          <div className={styles.logo}>{brandName}</div>
          {/*open && (
            <button className={styles.mobileToggle} onClick={closeMobile}>
              Закрыть
            </button>
          )*/}
        </div>
        <input
          className={styles.search}
          placeholder="Поиск по сайту"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <div className={styles.tree}>
          {query ? (
            <div className={styles.searchResults}>
              {results.length === 0 && <div style={{ color: "var(--muted)" }}>Ничего не найдено</div>}
              {results.map((note) => (
                <Link
                  key={note.slug}
                  href={`/${note.slug}`}
                  onClick={closeMobile}
                  className={`${styles.link} ${activeSlug === note.slug ? styles.active : ""}`}
                >
                  <div>{note.title}</div>
                  {note.tags.length > 0 && <div className={styles.tags}>{note.tags.join(", ")}</div>}
                </Link>
              ))}
            </div>
          ) : (
            renderFolder(rootNode)
          )}
        </div>
      </div>
    </aside>
  );
}
