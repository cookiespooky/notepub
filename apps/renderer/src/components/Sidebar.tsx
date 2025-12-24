"use client";

import { useEffect, useMemo, useState, type CSSProperties } from "react";
import styles from "./Sidebar.module.css";
import { getAppUrl } from "@/lib/domains";
import type { CategoryIndex, FlatNoteIndex } from "@/lib/types";
import { NavLink } from "@/components/NavContext";
import { useSidebarData } from "@/components/SidebarDataContext";

type SidebarProps = {
  siteSlug: string;
  categories: CategoryIndex[];
  flat: Omit<FlatNoteIndex, "key">[];
  activeSlug: string;
  activeCategorySlug?: string | null;
  siteTitle?: string;
  siteAvatarUrl?: string | null;
  onSelect?: (slug: string, relativeKey?: string) => void;
  headerSlot?: React.ReactNode;
  contentSlot?: React.ReactNode;
  onFolderRename?: (from: string, to: string) => Promise<void> | void;
  onFolderDelete?: (folder: string, hasNotes: boolean) => Promise<void> | void;
  onFileDelete?: (relativeKey: string, title?: string) => Promise<void> | void;
};

export function Sidebar({
  siteSlug,
  categories,
  flat,
  activeSlug,
  activeCategorySlug,
  siteTitle,
  siteAvatarUrl,
  onSelect,
  headerSlot,
  contentSlot,
  onFolderRename,
  onFolderDelete,
  onFileDelete,
}: SidebarProps) {
  const INDENT_STEP = 24;
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [openSections, setOpenSections] = useState<Set<string>>(new Set());
  const [closedSections, setClosedSections] = useState<Set<string>>(new Set());
  const [menuOpen, setMenuOpen] = useState<string | null>(null);
  const [pendingSlug, setPendingSlug] = useState<string | null>(null);
  const cached = useSidebarData(siteSlug, { categories, flat });
  const effectiveCategories = cached.categories;
  const effectiveFlat = cached.flat;
  const displayActiveSlug = pendingSlug || activeSlug;
  const slugToRelative = useMemo(() => {
    const map = new Map<string, string>();
    effectiveFlat.forEach((note) => {
      map.set(note.slug, note.relativeKey);
    });
    return map;
  }, [effectiveFlat]);
  const brandName = siteTitle?.trim() || "Notepub";
  const avatarSrc = siteAvatarUrl?.trim() || "/logo.png";
  const appUrl = getAppUrl();
  const rootNotes = useMemo(() => {
    return [...effectiveFlat.filter((note) => !note.category)].sort((a, b) => {
      if (a.isHome && !b.isHome) return -1;
      if (b.isHome && !a.isHome) return 1;
      return a.title.localeCompare(b.title, "ru");
    });
  }, [effectiveFlat]);

  const derivedActiveCategorySlug = useMemo(() => {
    if (activeCategorySlug) return activeCategorySlug;
    const found = effectiveCategories.find((cat) => cat.notes.some((note) => note.slug === displayActiveSlug));
    return found?.slug || null;
  }, [activeCategorySlug, effectiveCategories, displayActiveSlug]);

  const requiredOpenKeys = useMemo(() => {
    const paths = new Set<string>();
    if (derivedActiveCategorySlug) paths.add(derivedActiveCategorySlug);
    if (effectiveCategories.length === 1) paths.add(effectiveCategories[0].slug);
    return paths;
  }, [derivedActiveCategorySlug, effectiveCategories]);

  useEffect(() => {
    setClosedSections(new Set());
    setOpenSections(new Set(requiredOpenKeys));
  }, [activeSlug, requiredOpenKeys]);

  useEffect(() => {
    if (typeof document === "undefined") return;
    const body = document.body;
    const originalOverflow = body.style.overflow;
    if (open) {
      body.style.overflow = "hidden";
    } else {
      body.style.overflow = originalOverflow || "";
    }
    return () => {
      body.style.overflow = originalOverflow || "";
    };
  }, [open]);

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (!(e.target as HTMLElement)?.closest?.(`.${styles.folderMenu}`) && !(e.target as HTMLElement)?.closest?.(`.${styles.folderMenuButton}`)) {
        setMenuOpen(null);
      }
    };
    document.addEventListener("click", onDocClick);
    return () => document.removeEventListener("click", onDocClick);
  }, []);
  const effectiveOpen = useMemo(() => {
    const merged = new Set(openSections);
    requiredOpenKeys.forEach((key) => {
      if (!closedSections.has(key)) merged.add(key);
    });
    return merged;
  }, [openSections, requiredOpenKeys, closedSections]);

  useEffect(() => {
    // Clear optimistic state when navigation settles.
    setPendingSlug(null);
  }, [activeSlug]);

  const markActive = (slug: string) => {
    setPendingSlug(slug);
  };

  const getFolderFromPath = (path: string) => {
    const parts = path.split("/").filter(Boolean);
    if (parts.length <= 1) return "";
    return parts.slice(0, -1).join("/");
  };

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return [];
    return effectiveFlat
      .filter((note) => {
        const titleMatch = (note.title || "").toLowerCase().includes(q);
        const previewMatch = (note.preview || "").toLowerCase().includes(q);
        const slugMatch = (note.slug || "").toLowerCase().includes(q);
        const bodyMatch = (note.html || "").toLowerCase().includes(q);
        return titleMatch || previewMatch || slugMatch || bodyMatch;
      })
      .slice(0, 30);
  }, [effectiveFlat, query]);

  const closeMobile = () => setOpen(false);
  const toggleSection = (key: string, opts?: { forceOpen?: boolean }) => {
    setOpenSections((prev) => {
      const next = new Set(prev);
      if (opts?.forceOpen) {
        next.add(key);
        setClosedSections((prevClosed) => {
          const copy = new Set(prevClosed);
          copy.delete(key);
          return copy;
        });
        return next;
      }
      if (next.has(key)) {
        next.delete(key);
        setClosedSections((prevClosed) => {
          const copy = new Set(prevClosed);
          copy.add(key);
          return copy;
        });
      } else {
        next.add(key);
        setClosedSections((prevClosed) => {
          const copy = new Set(prevClosed);
          copy.delete(key);
          return copy;
        });
      }
      return next;
    });
  };

  const renderCategory = (category: CategoryIndex) => {
    const key = category.slug;
    const isOpen = effectiveOpen.has(key);
    const bodyClass = `${styles.sectionBody} ${isOpen ? styles.sectionBodyOpen : ""} ${styles.sectionBodyNested}`;
    const sectionClass = `${styles.section} ${styles.sectionNested} ${isOpen ? styles.sectionNestedOpen : ""}`;
    const sectionStyle = { ["--indent" as string]: `0px` } as CSSProperties;
    const label = category.name || "Без категории";
    return (
      <div key={key} className={sectionClass} style={sectionStyle}>
        <div className={styles.sectionHeaderRow}>
          <button className={styles.sectionHeader} type="button" onClick={() => toggleSection(key)}>
            <span className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ""}`}>▶</span>
            <span className={styles.sectionTitle}>{label}</span>
          </button>
          {onFolderRename && (
            <div className={styles.folderMenuWrapper}>
              <button
                className={styles.folderMenuButton}
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setMenuOpen(menuOpen === key ? null : key);
                }}
                aria-label="Действия с папкой"
              >
                ⋯
              </button>
              {menuOpen === key && (
                <div className={styles.folderMenu}>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      const next = prompt("Новое имя папки?", category.name);
                      if (!next || next.trim() === category.name) {
                        setMenuOpen(null);
                        return;
                      }
                      void onFolderRename?.(category.name, next.trim());
                      setMenuOpen(null);
                    }}
                  >
                    Переименовать
                  </button>
                  {onFolderDelete && (
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        const hasNotes = effectiveFlat.some((note) => {
                          const rel = note.relativeKey || "";
                          const folder = getFolderFromPath(rel);
                          return folder === category.name;
                        });
                        void onFolderDelete?.(category.name, hasNotes);
                        setMenuOpen(null);
                      }}
                    >
                      Удалить
                    </button>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
        <div className={bodyClass}>
          {category.notes.map((child) => (
            <div
              key={child.slug}
              className={`${styles.itemRowWithMenu} ${displayActiveSlug === child.slug ? styles.activeRow : ""}`}
              style={{ paddingLeft: INDENT_STEP }}
            >
              <NavLink
                href={`/${child.slug}`}
                onClick={(e) => {
                  markActive(child.slug);
                  if (onSelect) {
                    e.preventDefault();
                    onSelect(child.slug, slugToRelative.get(child.slug) || undefined);
                    closeMobile();
                  } else {
                    closeMobile();
                  }
                }}
                className={`${styles.link} ${displayActiveSlug === child.slug ? styles.active : ""} ${child.isDraft ? styles.draft : ""}`}
              >
                {child.title}
              </NavLink>
              {onFileDelete && (
                <div className={`${styles.folderMenuWrapper} ${styles.fileMenuWrapper}`}>
                  <button
                    className={styles.folderMenuButton}
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      setMenuOpen(menuOpen === `file-${child.slug}` ? null : `file-${child.slug}`);
                    }}
                    aria-label="Действия с файлом"
                  >
                    ⋯
                  </button>
                  {menuOpen === `file-${child.slug}` && (
                    <div className={styles.folderMenu}>
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          void onFileDelete?.(slugToRelative.get(child.slug) || "", child.title);
                          setMenuOpen(null);
                        }}
                      >
                        Удалить
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    );
  };

  return (
    <>
      <div className={styles.topBar}>
        <div className={styles.topBarInner}>
          <NavLink href="/" onClick={closeMobile} className={styles.brand}>
            <div className={styles.avatar}>
              <img src={avatarSrc} alt={brandName} className={styles.avatarImage} />
            </div>
            <div className={styles.logo}>{brandName}</div>
          </NavLink>
          <div className={styles.topActions}>
            {/*<div className={styles.userAvatar}>
              <img src="/logo.png" alt="User avatar" />
            </div>*/}
            <button
              className={`${styles.mobileToggle} ${open ? styles.mobileToggleOpen : ""}`}
              onClick={() => setOpen((prev) => !prev)}
              aria-label={open ? "Закрыть меню" : "Открыть меню"}
            >
              <span />
              <span />
              <span />
            </button>
          </div>
        </div>
      </div>
      <div className={`${styles.mobilePortal} ${open ? styles.visible : ""}`} onClick={closeMobile} />
      <aside className={`${styles.sidebar} ${open ? styles.sidebarOpen : ""}`}>
        <div className={`${styles.panel} ${open ? styles.open : ""}`}>
          {headerSlot}
          {contentSlot ? (
            contentSlot
          ) : (
            <>
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
                      <div
                        key={note.slug}
                        className={`${styles.itemRowWithMenu} ${displayActiveSlug === note.slug ? styles.activeRow : ""}`}
                      >
                        <NavLink
                          href={`/${note.slug}`}
                          onClick={(e) => {
                            markActive(note.slug);
                            if (onSelect) {
                              e.preventDefault();
                              onSelect(note.slug, slugToRelative.get(note.slug) || undefined);
                              closeMobile();
                            } else {
                              closeMobile();
                            }
                          }}
                          className={`${styles.link} ${displayActiveSlug === note.slug ? styles.active : ""} ${note.isDraft ? styles.draft : ""}`}
                        >
                          <div>{note.title}</div>
                          {note.tags.length > 0 && <div className={styles.tags}>{note.tags.join(", ")}</div>}
                        </NavLink>
                        {onFileDelete && (
                          <div className={`${styles.folderMenuWrapper} ${styles.fileMenuWrapper}`}>
                            <button
                              className={styles.folderMenuButton}
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                setMenuOpen(menuOpen === `file-${note.slug}` ? null : `file-${note.slug}`);
                              }}
                              aria-label="Действия с файлом"
                            >
                              ⋯
                            </button>
                            {menuOpen === `file-${note.slug}` && (
                              <div className={styles.folderMenu}>
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    void onFileDelete?.(slugToRelative.get(note.slug) || "", note.title);
                                    setMenuOpen(null);
                                  }}
                                >
                                  Удалить
                                </button>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <>
                    {rootNotes.map((note) => (
                      <div
                        key={note.slug}
                        className={`${styles.itemRowWithMenu} ${displayActiveSlug === note.slug ? styles.activeRow : ""}`}
                      >
                        <NavLink
                          href={`/${note.slug}`}
                          onClick={(e) => {
                            markActive(note.slug);
                            if (onSelect) {
                              e.preventDefault();
                              onSelect(note.slug, slugToRelative.get(note.slug) || undefined);
                              closeMobile();
                            } else {
                              closeMobile();
                            }
                          }}
                          className={`${styles.link} ${displayActiveSlug === note.slug ? styles.active : ""} ${note.isDraft ? styles.draft : ""}`}
                        >
                          {note.title}
                        </NavLink>
                        {onFileDelete && (
                          <div className={`${styles.folderMenuWrapper} ${styles.fileMenuWrapper}`}>
                            <button
                              className={styles.folderMenuButton}
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                setMenuOpen(menuOpen === `file-${note.slug}` ? null : `file-${note.slug}`);
                              }}
                              aria-label="Действия с файлом"
                            >
                              ⋯
                            </button>
                            {menuOpen === `file-${note.slug}` && (
                              <div className={styles.folderMenu}>
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    void onFileDelete?.(slugToRelative.get(note.slug) || "", note.title);
                                    setMenuOpen(null);
                                  }}
                                >
                                  Удалить
                                </button>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    ))}
                    {effectiveCategories.map((category) => renderCategory(category))}
                  </>
                )}
              </div>
              <NavLink href={appUrl} className={styles.badge} target="_blank" rel="noreferrer">
                <span>Сделано на Notepub</span>
              </NavLink>
            </>
          )}
        </div>
      </aside>
    </>
  );
}
