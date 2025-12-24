"use client";

import { createContext, useContext, useRef } from "react";
import type { CategoryIndex, FlatNoteIndex } from "@/lib/types";

type SidebarData = { categories: CategoryIndex[]; flat: Omit<FlatNoteIndex, "key">[] };
type SidebarDataEntry = { data: SidebarData; signature: string };
type SidebarDataMap = Map<string, SidebarDataEntry>;

const SidebarDataContext = createContext<SidebarDataMap | null>(null);

export function SidebarDataProvider({ children }: { children: React.ReactNode }) {
  const storeRef = useRef<SidebarDataMap>(new Map());
  const value = storeRef.current;
  return <SidebarDataContext.Provider value={value}>{children}</SidebarDataContext.Provider>;
}

export function useSidebarData(siteSlug: string, initial: SidebarData) {
  const store = useContext(SidebarDataContext);
  if (!store) {
    return initial;
  }
  const signature = buildSignature(initial);
  const existing = store.get(siteSlug);
  if (!existing || existing.signature !== signature) {
    store.set(siteSlug, { data: initial, signature });
  }
  return (store.get(siteSlug) as SidebarDataEntry).data;
}

function buildSignature(data: SidebarData) {
  const cats = data.categories
    .map((cat) => `${cat.slug}:${cat.name}:${cat.notes.length}`)
    .join("|");
  const flat = data.flat
    .map((note) => `${note.slug}:${note.title}:${note.category || ""}:${note.isDraft ? 1 : 0}`)
    .join("|");
  return `${cats}__${flat}`;
}
