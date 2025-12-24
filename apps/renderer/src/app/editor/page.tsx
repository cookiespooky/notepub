"use client";

/* Editor UI scaffold: uses real tree data, autosave, and shared hooks. */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type React from "react";
import { EditorSelection, EditorState, RangeSetBuilder } from "@codemirror/state";
import {
  Decoration,
  EditorView,
  WidgetType,
  drawSelection,
  dropCursor,
  highlightActiveLine,
  keymap,
  placeholder as cmPlaceholder,
  ViewPlugin,
  ViewUpdate,
} from "@codemirror/view";
import {
  defaultHighlightStyle,
  syntaxHighlighting,
  bracketMatching,
} from "@codemirror/language";
import { markdown, markdownLanguage, markdownKeymap } from "@codemirror/lang-markdown";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
  undo,
  redo,
} from "@codemirror/commands";
import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
  type Completion,
  type CompletionContext,
} from "@codemirror/autocomplete";
import FormatBoldRoundedIcon from "@mui/icons-material/FormatBoldRounded";
import FormatItalicRoundedIcon from "@mui/icons-material/FormatItalicRounded";
import CodeRoundedIcon from "@mui/icons-material/CodeRounded";
import InsertLinkRoundedIcon from "@mui/icons-material/InsertLinkRounded";
import FormatListNumberedRoundedIcon from "@mui/icons-material/FormatListNumberedRounded";
import FormatListBulletedRoundedIcon from "@mui/icons-material/FormatListBulletedRounded";
import FormatQuoteRoundedIcon from "@mui/icons-material/FormatQuoteRounded";
import InsertPhotoRoundedIcon from "@mui/icons-material/InsertPhotoRounded";
import UndoRoundedIcon from "@mui/icons-material/UndoRounded";
import RedoRoundedIcon from "@mui/icons-material/RedoRounded";
import OpenInNewRoundedIcon from "@mui/icons-material/OpenInNewRounded";
import { DEFAULT_THEME, type ThemeSettings } from "@/lib/theme";
import { Sidebar } from "@/components/Sidebar";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { useAutosaveNote, useNoteLinks } from "@notepub/ui";
import { slugifySegment } from "@/lib/slug";
import styles from "./page.module.css";

declare global {
  interface Window {
    __EDITOR_THEME?: CssSettings;
  }
}

type Frontmatter = {
  title: string;
  slug: string;
  category?: string;
  home?: boolean;
  draft?: boolean;
  [key: string]: unknown;
};

type NoteRecord = {
  path: string;
  slug: string;
  title: string;
  category: string | null;
  relativeKey?: string;
  home?: boolean;
  draft?: boolean;
};

type ToolbarAction =
  | "bold"
  | "italic"
  | "code"
  | "h1"
  | "h2"
  | "h3"
  | "h4"
  | "text"
  | "remove"
  | "bullet"
  | "ordered"
  | "quote"
  | "link"
  | "undo"
  | "redo";

const DEFAULT_FRONTMATTER: Frontmatter = { title: "", slug: "", category: "", draft: true, home: false };
let notePathForWidgets: string | null = null;
type CssSettings = ThemeSettings;

const DEFAULT_CSS_SETTINGS: CssSettings = { ...DEFAULT_THEME };

export default function EditorPage() {
  const [notes, setNotes] = useState<NoteRecord[]>([]);
  const [activePath, setActivePath] = useState<string | null>(null);
  const [frontmatter, setFrontmatter] = useState<Frontmatter>(DEFAULT_FRONTMATTER);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [body, setBody] = useState("");
  const [mode, setMode] = useState<"markdown" | "preview">("markdown");
  const [loading, setLoading] = useState(false);
  const [previewHtml, setPreviewHtml] = useState<string>("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [pagesError, setPagesError] = useState<string | null>(null);
  const [sidebarTab, setSidebarTab] = useState<"content" | "css">("content");
  const [accent, setAccent] = useState<string>("");
  const [deleting, setDeleting] = useState(false);
  const [creatingFolder, setCreatingFolder] = useState(false);
  const [creatingPage, setCreatingPage] = useState(false);
  const [selectedFolder, setSelectedFolder] = useState<string>("");
  const [extraFolders, setExtraFolders] = useState<Set<string>>(new Set());
  const [loadingTree, setLoadingTree] = useState(false);
  const [cssSettings, setCssSettings] = useState<CssSettings | null>(null);
  const [cssLoading, setCssLoading] = useState(false);
  const [cssSaving, setCssSaving] = useState(false);
  const [cssStatus, setCssStatus] = useState<"idle" | "saved" | "error">("idle");
  const [showBubble, setShowBubble] = useState(false);
  const [bubbleCoords, setBubbleCoords] = useState<{ left: number; top: number } | null>(null);
  const [plusCoords, setPlusCoords] = useState<{ left: number; top: number } | null>(null);
  const [plusOpen, setPlusOpen] = useState(false);
  const [imageInserting, setImageInserting] = useState(false);
  const [deletingImageSrc, setDeletingImageSrc] = useState<string | null>(null);
  const onBodyChangeRef = useRef<(value: string) => void>(() => {});
  const bodyRef = useRef(body);
  const bubbleRef = useRef<HTMLDivElement | null>(null);
  const plusMenuRef = useRef<HTMLDivElement | null>(null);
  const cssLoadedRef = useRef(false);
  const editorParentRef = useRef<HTMLDivElement | null>(null);
  const editorAreaRef = useRef<HTMLDivElement | null>(null);
  const editorSurfaceRef = useRef<HTMLDivElement | null>(null);
  const editorViewRef = useRef<EditorView | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.title = "Редактор";
    }
  }, []);

  useEffect(() => {
    if (!plusOpen) return;
    const handleClick = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (plusMenuRef.current && target && plusMenuRef.current.contains(target)) return;
      setPlusOpen(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [plusOpen]);

  // Fetch tree
  useEffect(() => {
    void (async () => {
      setLoadingTree(true);
      try {
        const res = await fetch("/api/editor/pages");
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
        setPagesError(data.error || `Не удалось загрузить страницы (${res.status})`);
        return;
      }
        const data = await res.json();
        setNotes(data.pages || []);
        setExtraFolders(new Set(data.folders || []));
        setPagesError(null);
        if (!activePath && data.pages?.[0]) {
          loadNoteByPath(data.pages[0].path);
        }
      } finally {
        setLoadingTree(false);
      }
    })();
  }, []);

  // Initialize accent from CSS vars
  useEffect(() => {
    if (typeof window === "undefined") return;
    const style = getComputedStyle(document.documentElement);
    const current = style.getPropertyValue("--color-accent").trim();
    if (current) setAccent(current);
  }, []);

  // Load CSS settings
  useEffect(() => {
    void (async () => {
      setCssLoading(true);
      try {
      const res = await fetch("/api/editor/css");
      if (!res.ok) return;
      const data = await res.json();
      if (data?.settings && Object.keys(data.settings).length > 0) {
        const merged = { ...resolveInitialCssSettings(), ...data.settings };
        setCssSettings(merged);
        setAccent(merged.primary || "");
        cssLoadedRef.current = true;
      }
    } finally {
      setCssLoading(false);
    }
  })();
  }, []);

  useEffect(() => {
    if (cssSettings?.primary) {
      setAccent(cssSettings.primary);
    }
  }, [cssSettings?.primary]);

  // Force preview mode when switching to CSS tab
  useEffect(() => {
    if (sidebarTab === "css" && mode !== "preview") {
      setMode("preview");
    }
  }, [sidebarTab, mode]);

  const refreshTree = useCallback(async () => {
    setLoadingTree(true);
    try {
      const res = await fetch("/api/editor/pages");
      if (!res.ok) return [];
      const data = await res.json();
      setNotes(data.pages || []);
      setExtraFolders(new Set(data.folders || []));
      return data.pages || [];
    } finally {
      setLoadingTree(false);
    }
  }, []);

  const saveCssSettings = useCallback(
    async (settingsOverride?: CssSettings) => {
      if (!cssSettings && !settingsOverride) return;
      setCssSaving(true);
      setCssStatus("idle");
      try {
        const payload = (settingsOverride || cssSettings)!;
        const res = await fetch("/api/editor/css", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ settings: payload }),
        });
        if (!res.ok) {
        throw new Error("Не удалось сохранить");
      }
        setCssStatus("saved");
        setAccent(payload.primary);
        applyDesignTokens(payload);
      } catch {
        setCssStatus("error");
      } finally {
        setCssSaving(false);
      }
    },
    [cssSettings],
  );

  const linkHook = useNoteLinks({
    notes: notes.map((n) => ({ slug: n.slug, title: n.title, category: n.category || undefined })),
    onNavigate: (slug) => {
      const match = notes.find((n) => n.slug === slug);
      if (match) loadNoteByPath(match.path);
    },
  });

  const { note, setNote, dirty, saving, error, lastSavedAt, triggerSaveNow } = useAutosaveNote({
    note: { path: activePath || "", frontmatter, body },
    validate: (n) => {
      if (!n.frontmatter.title?.trim()) return { ok: false, error: "Нужно указать заголовок" };
      return { ok: true };
    },
    save: async (n) => {
      const fallbackFolder = getFolderFromPath(activePath || "");
      const targetPath = buildPath(n.frontmatter.title || "Без названия", selectedFolder || fallbackFolder);
      const payload = { ...n, path: activePath || n.path, targetPath };
      setSaveError(null);
      const res = await fetch("/api/editor/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        const conflict = res.status === 409;
        const message = conflict ? "Адрес уже занят, выберите другой" : data.error || "Не удалось сохранить";
        setSaveError(message);
        throw new Error(message);
      }
      const data = await res.json();
      setActivePath(data.path);
      setSelectedFolder(getFolderFromPath(data.path));
      void refreshTree();
      setSaveError(null);
      return data;
    },
    publishInFlight: deleting,
  });

  const [fallbackTime, setFallbackTime] = useState<Date | null>(null);
  useEffect(() => {
    setFallbackTime(new Date());
  }, []);

  const statusInfo = useMemo(() => {
    if (saving) return { state: "saving" as const, label: "Сохранение…", time: lastSavedAt };
    if (error) return { state: "error" as const, label: error, time: lastSavedAt };
    if (dirty) return { state: "unsaved" as const, label: "Есть изменения", time: lastSavedAt };
    if (lastSavedAt) return { state: "saved" as const, label: "Сохранено", time: lastSavedAt };
    return { state: "saved" as const, label: "Актуально", time: null as Date | null };
  }, [dirty, saving, lastSavedAt, error]);
  const derivedSlugError = !frontmatter.slug?.trim() ? "Для публикации необходимо заполнить адрес" : null;
  const topError = saveError || error || derivedSlugError || pagesError;

  const modeRef = useRef(mode);
  useEffect(() => {
    modeRef.current = mode;
    if (mode !== "markdown") {
      setShowBubble(false);
    }
  }, [mode]);

  const linkCompletions = useMemo<Completion[]>(() => {
    return notes.map((n) => ({
      label: n.title || n.slug,
      detail: n.slug,
      type: "keyword",
      info: n.relativeKey ? `/${n.relativeKey}` : undefined,
      apply(view, completion, from, to) {
        const slug = (completion.detail as string) || completion.label;
        const hasClosing = view.state.sliceDoc(to, to + 2) === "]]";
        const insert = hasClosing ? slug : `${slug}]]`;
        const anchor = from + insert.length;
        view.dispatch({
          changes: { from, to, insert },
          selection: { anchor },
        });
      },
    }));
  }, [notes]);

  const linkCompletionSource = useMemo(
    () => (context: CompletionContext) => {
      if (modeRef.current !== "markdown") return null;
      const match = context.matchBefore(/\[\[[^\]]*(\]\])?/);
      if (!match) return null;
      // Allow explicit trigger right after typing [[
      if (!context.explicit && match.from === match.to) return null;
      const hasClosing = match.text.endsWith("]]");
      const raw = hasClosing ? match.text.slice(2, -2) : match.text.slice(2); // drop [[ and closing if present
      const query = raw.toLowerCase();
      const options = linkCompletions.filter((opt) => {
        const label = (opt.label || "").toLowerCase();
        const detail = (opt.detail || "").toLowerCase();
        return label.includes(query) || detail.includes(query);
      });
      return {
        from: match.from + 2,
        to: hasClosing ? match.to - 2 : match.to,
        options: options.length > 0 ? options : linkCompletions,
        validFor: /^[^\]]*$/, // keep suggestions open while the user edits inside [[...]]
      };
    },
    [linkCompletions],
  );

  const editorExtensions = useMemo(
    () => [
      markdown({ base: markdownLanguage }),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      history(),
      drawSelection(),
      dropCursor(),
      highlightActiveLine(),
      bracketMatching(),
      closeBrackets(),
      autocompletion({ override: [linkCompletionSource], icons: false }),
      EditorView.lineWrapping,
      cmPlaceholder("Введите текст…"),
      keymap.of([
        indentWithTab,
        ...closeBracketsKeymap,
        ...completionKeymap,
        ...defaultKeymap,
        ...historyKeymap,
        ...markdownKeymap,
      ]),
      EditorView.theme(
        {
          "&": {
            backgroundColor: "transparent",
            color: "var(--color-text)",
            fontFamily: "var(--design-font, var(--font-body))",
            fontSize: "var(--design-font-size, 16px)",
          },
          ".cm-scroller": {
            fontFamily: "inherit",
            lineHeight: "1.7",
            fontSize: "inherit",
          },
          ".cm-content": {
            padding: "4px 0",
            fontFamily: "inherit",
            fontSize: "inherit",
            lineHeight: "inherit",
          },
          ".cm-line": {
            padding: "8px 4px 8px 8px",
            fontSize: "inherit",
            lineHeight: "var(--line-body, 1.7)",
          },
          ".cm-selectionBackground": {
            background: "rgba(12, 74, 110, 0.18)",
          },
          "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground": {
            background: "rgba(12, 74, 110, 0.26)",
          },
          ".cm-gutters": {
            backgroundColor: "transparent",
            border: "none",
          },
          "&.cm-editor.cm-focused": {
            outline: "none",
          },
          ".cm-activeLine": {
            backgroundColor: "var(--color-surface-muted, #f1f5f9)",
          },

        },
        { dark: false },
      ),
      markdownLineDecorations,
      imageDataUrlPreviewPlugin,
    ],
    [linkCompletionSource],
  );

  const withView = useCallback((fn: (view: EditorView) => void) => {
    const view = editorViewRef.current;
    if (!view) return;
    fn(view);
  }, []);

  const updateBubblePosition = useCallback((view: EditorView) => {
    if (modeRef.current !== "markdown") {
      setShowBubble(false);
      return;
    }
    const sel = view.state.selection.main;
    if (sel.empty) {
      setShowBubble(false);
      return;
    }
    const host = editorAreaRef.current;
    if (!host) return;
    const from = Math.min(sel.from, sel.to);
    const to = Math.max(sel.from, sel.to);
    const fromCoords = view.coordsAtPos(from);
    const toCoords = view.coordsAtPos(to);
    if (!fromCoords || !toCoords) {
      setShowBubble(false);
      return;
    }
    const hostRect = host.getBoundingClientRect();
    const bubbleWidth = bubbleRef.current?.offsetWidth ?? 0;
    const halfWidth = bubbleWidth ? bubbleWidth / 2 : 0;
    const centerLeft = (fromCoords.left + toCoords.right) / 2 - hostRect.left;
    const minLeft = 12 + halfWidth;
    const maxLeft = hostRect.width - halfWidth - 12;
    const clampedLeft = Math.max(minLeft, Math.min(centerLeft, maxLeft));
    const rawTop = Math.min(fromCoords.top, toCoords.top) - hostRect.top - 12;
    const viewportTop = Math.max(8, rawTop);
    setBubbleCoords({ left: clampedLeft, top: viewportTop });
    setShowBubble(true);
  }, []);

  const updatePlusPosition = useCallback(
    (view: EditorView) => {
      if (modeRef.current !== "markdown") {
        setPlusCoords(null);
        setPlusOpen(false);
        return;
      }
      const host = editorSurfaceRef.current;
      if (!host) return;
      const line = view.state.doc.lineAt(view.state.selection.main.head);
      const lineStart = line.from;
      const coords = view.coordsAtPos(lineStart);
      if (!coords) return;
      const hostRect = host.getBoundingClientRect();
      const left = 16;
      const top = coords.top - hostRect.top - 7;
      setPlusCoords({ left, top });
    },
    [],
  );

  const uploadImage = useCallback(async (file: File) => {
    const form = new FormData();
    form.append("file", file);
    const currentFolder = selectedFolder || getFolderFromPath(activePath || "");
    if (currentFolder) {
      form.append("folder", currentFolder);
    }
    const res = await fetch("/api/editor/upload", {
      method: "POST",
      body: form,
    });
    if (!res.ok) throw new Error("upload failed");
    const data = await res.json();
    return { url: data.url as string, key: data.key as string };
  }, [activePath, selectedFolder]);

  const processImageFile = useCallback(
    async (view: EditorView, file: File) => {
      if (!file.type.startsWith("image/")) return;
      setImageInserting(true);
      try {
        const { key } = await uploadImage(file);
        const displayName = file.name || key || "image";
        insertImageMarkdown(view, key, displayName);
        updateBubblePosition(view);
        updatePlusPosition(view);
        view.focus();
      } catch {
        // fallback: embed data URL so user doesn't lose content
        const reader = new FileReader();
        reader.onload = () => {
          const dataUrl = typeof reader.result === "string" ? reader.result : "";
          if (dataUrl) {
            insertImageMarkdown(view, dataUrl, file.name);
            updateBubblePosition(view);
            updatePlusPosition(view);
            view.focus();
          }
          setImageInserting(false);
        };
        reader.onerror = () => setImageInserting(false);
        reader.readAsDataURL(file);
        return;
      }
      setImageInserting(false);
    },
    [updateBubblePosition, uploadImage],
  );

  const handleFileList = useCallback(
    (view: EditorView, fileList?: FileList | null) => {
      if (!fileList || fileList.length === 0) return false;
      const firstImage = Array.from(fileList).find((f) => f.type.startsWith("image/"));
      if (!firstImage) return false;
      processImageFile(view, firstImage);
      return true;
    },
    [processImageFile],
  );

  // Sync note state when autosave hook internal note changes
  useEffect(() => {
    setFrontmatter(note.frontmatter as Frontmatter);
    setBody(note.body);
  }, [note]);

  // Keep folder selection aligned with the active path
  useEffect(() => {
    if (!activePath) return;
    const folder = getFolderFromPath(activePath);
    setSelectedFolder(folder);
    notePathForWidgets = activePath;
  }, [activePath]);

  // Preview render using server-side markdown pipeline for parity with renderer.
  useEffect(() => {
    if (mode !== "preview") return;
    const controller = new AbortController();
    const run = async () => {
      setPreviewLoading(true);
      try {
        const res = await fetch("/api/editor/preview", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ body, objectKey: activePath || "" }),
          signal: controller.signal,
        });
        if (!res.ok) return;
        const data = await res.json();
        setPreviewHtml(data.html || "");
      } finally {
        setPreviewLoading(false);
      }
    };
    const timer = setTimeout(() => {
      void run();
    }, 400);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [mode, body, activePath]);

  useEffect(() => {
    if (mode !== "markdown") {
      if (editorViewRef.current) {
        editorViewRef.current.destroy();
        editorViewRef.current = null;
      }
      setShowBubble(false);
      return;
    }
    if (!editorParentRef.current) return;
    const view = new EditorView({
      state: EditorState.create({
        doc: bodyRef.current,
        extensions: [
          editorExtensions,
          EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          const next = update.state.doc.toString();
          if (next !== bodyRef.current) {
            onBodyChangeRef.current(next);
            bodyRef.current = next;
          }
        }
        if (update.selectionSet || update.focusChanged) {
          updateBubblePosition(update.view);
          updatePlusPosition(update.view);
        }
      }),
          EditorView.domEventHandlers({
            paste(event, view) {
              const handled = handleFileList(view, event.clipboardData?.files);
              if (handled) {
                event.preventDefault();
                return true;
              }
              return false;
            },
            drop(event, view) {
              const handled = handleFileList(view, event.dataTransfer?.files);
              if (handled) {
                event.preventDefault();
                return true;
              }
              return false;
            },
            scroll() {
              const nextView = editorViewRef.current;
              if (nextView) {
                updateBubblePosition(nextView);
                updatePlusPosition(nextView);
              }
              return false;
            },
          }),
        ],
      }),
      parent: editorParentRef.current,
    });
    editorViewRef.current = view;
    updateBubblePosition(view);
    return () => {
      view.destroy();
      editorViewRef.current = null;
    };
  }, [editorExtensions, handleFileList, mode, updateBubblePosition]);

  useEffect(() => {
    const view = editorViewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (body === current) return;
    view.dispatch({ changes: { from: 0, to: current.length, insert: body } });
    bodyRef.current = body;
    updateBubblePosition(view);
    updatePlusPosition(view);
  }, [body, updateBubblePosition]);

  const loadNoteByPath = async (path: string) => {
    setLoading(true);
    try {
      const res = await fetch(`/api/editor/page?path=${encodeURIComponent(path)}`);
      if (!res.ok) return;
      const data = await res.json();
      const fm = {
        title: data.frontmatter.title || "",
        slug: data.frontmatter.slug || "",
        category: "",
        home: Boolean(data.frontmatter.home),
        draft: data.frontmatter.draft !== false,
        ...data.frontmatter,
      } as Frontmatter;
      const bodyValue = data.body || "";
      setActivePath(data.path);
      setFrontmatter(fm);
      setBody(bodyValue);
      setNote({ path: data.path, frontmatter: fm, body: bodyValue });
      setSelectedFolder(getFolderFromPath(data.path));
    } finally {
      setLoading(false);
    }
  };

  const addPage = async () => {
    const currentFolder = selectedFolder || getFolderFromPath(activePath || "");
    const { title, slug, path } = generateDefaultPageIdentifiers(notes, currentFolder);
    setCreatingPage(true);
    try {
      const res = await fetch("/api/editor/add", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          frontmatter: { title, slug, draft: true },
          body: "",
          path,
        }),
      });
      if (!res.ok) {
        alert("Не удалось создать страницу");
        return;
      }
      const data = await res.json();
      await refreshTree();
      await loadNoteByPath(data.path);
    } finally {
      setCreatingPage(false);
    }
  };

  const addFolder = async () => {
    const normalized = generateDefaultFolderName(folderOptions, extraFolders);
    setCreatingFolder(true);
    try {
      const res = await fetch("/api/editor/folder", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: normalized }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        alert(data.error || "Не удалось создать папку");
        return;
      }
      setExtraFolders((prev) => new Set(prev).add(normalized));
      setSelectedFolder(normalized);
      await refreshTree();
    } finally {
      setCreatingFolder(false);
    }
  };

  const deletePage = async () => {
    const currentPath = activePath ?? note.path ?? "";
    if (!currentPath) {
      alert("Страница не выбрана");
      return;
    }
    const label = (frontmatter.title || activeNote?.title || frontmatter.slug || "").trim() || "эту страницу";
    const confirmed = confirm(`Удалить "${label}"? Действие необратимо.`);
    if (!confirmed) return;
    setDeleting(true);
    try {
      const res = await fetch("/api/editor/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: currentPath }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        alert(data.error || "Не удалось удалить страницу");
        return;
      }
      const pages = await refreshTree();
      const next = pages?.find((p: NoteRecord) => p.path !== currentPath) || pages?.[0];
      if (next) {
        await loadNoteByPath(next.path);
      } else {
        setActivePath(null);
        setFrontmatter(DEFAULT_FRONTMATTER);
        setBody("");
        setNote({ path: "", frontmatter: DEFAULT_FRONTMATTER, body: "" });
        setMode("markdown");
      }
    } finally {
      setDeleting(false);
    }
  };

  const onFieldChange = (key: keyof Frontmatter, value: any) => {
    const updated = { ...frontmatter, [key]: value };
    const currentPath = activePath ?? note.path ?? "";
    setFrontmatter(updated);
    setNote({ path: currentPath, frontmatter: updated, body });
  };

  const onBodyChange = useCallback(
    (value: string) => {
      const currentPath = activePath ?? note.path ?? "";
      setBody(value);
      setNote({ path: currentPath, frontmatter, body: value });
    },
    [activePath, note.path, frontmatter, setNote],
  );

  useEffect(() => {
    onBodyChangeRef.current = onBodyChange;
  }, [onBodyChange]);

  useEffect(() => {
    bodyRef.current = body;
  }, [body]);

  useEffect(() => {
    if (cssSettings) return;
    if (typeof window === "undefined") return;
    if (window.__EDITOR_THEME) {
      setCssSettings(window.__EDITOR_THEME);
      setAccent(window.__EDITOR_THEME.primary || "");
      cssLoadedRef.current = true;
      return;
    }
    const baseline = resolveInitialCssSettings();
    setCssSettings((current) => current || baseline);
    setAccent((current) => current || baseline.primary || "");
  }, [cssSettings]);

  useEffect(() => {
    if (!cssSettings) return;
    applyDesignTokens(cssSettings);
  }, [cssSettings]);

  const folderOptions = useMemo(() => {
    const set = new Set<string>();
    notes.forEach((n) => {
      const folder = getFolderFromPath(n.path);
      if (folder && folder !== ".keep") set.add(folder);
    });
    extraFolders.forEach((f) => {
      if (f && f !== ".keep") set.add(f);
    });
    if (selectedFolder && selectedFolder !== ".keep") set.add(selectedFolder);
    return Array.from(set).sort((a, b) => a.localeCompare(b, "ru"));
  }, [notes, extraFolders, selectedFolder]);

  const activeNote = useMemo(() => notes.find((n) => n.path === activePath), [notes, activePath]);
  const categories = useMemo(() => buildCategories(notes, extraFolders), [notes, extraFolders]);

  const handlePreviewClick: React.MouseEventHandler<HTMLDivElement> = (event) => {
    const anchor = (event.target as HTMLElement)?.closest("a") as HTMLAnchorElement | null;
    if (!anchor || !anchor.href) return;
    const link = linkHook.resolveHref(anchor.getAttribute("href") || anchor.href);
    linkHook.handleClick(event as any, link);
  };

  const accentVars = useMemo<React.CSSProperties | undefined>(
    () =>
      accent
        ? ({
            "--color-accent": accent,
            "--color-accent-strong": accent,
            "--color-accent-stronger": accent,
          } as React.CSSProperties)
        : undefined,
    [accent],
  );

  const effectiveCss = cssSettings || DEFAULT_CSS_SETTINGS;
  const openPublished = () => {
    const slug = (frontmatter.slug || activeNote?.slug || "").trim();
    const path = slug ? `/${slug}` : "/";
    const origin = typeof window !== "undefined" ? window.location.origin : "";
    const url = origin ? `${origin}${path}` : path;
    window.open(url, "_blank", "noopener,noreferrer");
  };

  const handleToolbarAction = useCallback(
    (action: ToolbarAction) => {
      withView((view) => {
        let changed = false;
        switch (action) {
          case "undo":
            changed = undo(view);
            break;
          case "redo":
            changed = redo(view);
            break;
          case "bold":
            wrapSelectionWith(view, "**", "**", "жирный текст");
            changed = true;
            break;
          case "italic":
            wrapSelectionWith(view, "_", "_", "курсив");
            changed = true;
            break;
          case "code":
            wrapSelectionWith(view, "`", "`", "код");
            changed = true;
            break;
          case "h1":
            applyBlockPrefix(view, "h1");
            changed = true;
            break;
          case "h2":
            applyBlockPrefix(view, "h2");
            changed = true;
            break;
          case "h3":
            applyBlockPrefix(view, "h3");
            changed = true;
            break;
          case "h4":
            applyBlockPrefix(view, "h4");
            changed = true;
            break;
          case "text":
            applyBlockPrefix(view, "text");
            changed = true;
            break;
          case "remove":
            removeLine(view);
            changed = true;
            break;
          case "bullet":
            applyBlockPrefix(view, "bullet");
            changed = true;
            break;
          case "ordered":
            applyBlockPrefix(view, "ordered");
            changed = true;
            break;
          case "quote":
            applyBlockPrefix(view, "quote");
            changed = true;
            break;
          case "link": {
            const url = prompt("Ссылка (URL)?");
            if (!url) return;
            insertLinkMarkdown(view, url);
            changed = true;
            break;
          }
        }
        if (changed) {
          updateBubblePosition(view);
          updatePlusPosition(view);
          view.focus();
        }
      });
    },
    [updateBubblePosition, updatePlusPosition, withView],
  );

  const handleImageFromUrl = useCallback(() => {
    const url = prompt("Ссылка на изображение");
    if (!url) return;
    withView((view) => {
      insertImageMarkdown(view, url);
      updateBubblePosition(view);
      updatePlusPosition(view);
      view.focus();
    });
  }, [updateBubblePosition, updatePlusPosition, withView]);

  const handleImagePicker = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleFileInputChange: React.ChangeEventHandler<HTMLInputElement> = useCallback(
    (event) => {
      const file = event.target.files?.[0];
      const view = editorViewRef.current;
      if (view && file) {
        processImageFile(view, file);
      }
      event.target.value = "";
    },
    [processImageFile],
  );

  return (
    <div
      className={styles.shell}
      style={accentVars}
    >
      <div>
        <Sidebar
          siteSlug="editor"
          categories={categories}
          flat={notes.map((n) => ({
            title: n.title,
            slug: n.slug,
            preview: "",
            category: n.category || null,
            breadcrumbs: [],
            isDraft: n.draft,
            relativeKey: n.path,
            categorySlug: n.category ? slugifySegment(n.category) || null : null,
            created: null,
            updated: null,
            etag: "",
            html: "",
            isHome: n.home,
            tags: [],
          }))}
          activeSlug={activeNote?.slug || ""}
          activeCategorySlug={activeNote?.category ? slugifySegment(activeNote.category) || undefined : undefined}
          siteTitle="Редактор"
          siteAvatarUrl={null}
          onSelect={(_slug, relativeKey) => {
            if (relativeKey) loadNoteByPath(relativeKey);
          }}
          onFolderRename={async (from, to) => {
            const normalizedFrom = (from || "").trim();
            const normalizedTo = (to || "").trim();
            if (!normalizedFrom || !normalizedTo) return;
            const res = await fetch("/api/editor/folder/rename", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ from: normalizedFrom, to: normalizedTo }),
            });
            if (!res.ok) {
              const data = await res.json().catch(() => ({}));
              alert(data.error || "Не удалось переименовать папку");
              return;
            }
            await refreshTree();
            if (activePath && activePath.startsWith(`${normalizedFrom}/`)) {
              const rest = activePath.slice(normalizedFrom.length + 1);
              const nextPath = `${normalizedTo}/${rest}`;
              await loadNoteByPath(nextPath);
            }
          }}
          onFolderDelete={async (folder, hasNotes) => {
            const confirmText = hasNotes
              ? `Удалить папку "${folder}" и все файлы внутри?`
              : `Удалить пустую папку "${folder}"?`;
            const ok = confirm(confirmText);
            if (!ok) return;
            setLoading(true);
            const res = await fetch("/api/editor/folder/delete", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ path: folder }),
            });
            setLoading(false);
            if (!res.ok) {
              const data = await res.json().catch(() => ({}));
              alert(data.error || "Не удалось удалить папку");
              return;
            }
            const pages = await refreshTree();
            if (activePath && activePath.startsWith(`${folder}/`)) {
              const next = pages?.find((p: NoteRecord) => !p.path.startsWith(`${folder}/`));
              if (next) {
                await loadNoteByPath(next.path);
              } else {
                setActivePath(null);
                setFrontmatter(DEFAULT_FRONTMATTER);
                setBody("");
                setNote({ path: "", frontmatter: DEFAULT_FRONTMATTER, body: "" });
                setMode("markdown");
              }
            }
          }}
          onFileDelete={async (relativeKey, title) => {
            const label = title?.trim() || relativeKey || "эту страницу";
            const ok = confirm(`Удалить "${label}"? Действие необратимо.`);
            if (!ok) return;
            setDeleting(true);
            try {
              const res = await fetch("/api/editor/delete", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ path: relativeKey }),
              });
              if (!res.ok) {
                const data = await res.json().catch(() => ({}));
                alert(data.error || "Не удалось удалить страницу");
                return;
              }
              const pages = await refreshTree();
              if (activePath && activePath === relativeKey) {
                const next = pages?.find((p: NoteRecord) => p.path !== relativeKey);
                if (next) {
                  await loadNoteByPath(next.path);
                } else {
                  setActivePath(null);
                  setFrontmatter(DEFAULT_FRONTMATTER);
                  setBody("");
                  setNote({ path: "", frontmatter: DEFAULT_FRONTMATTER, body: "" });
                  setMode("markdown");
                }
              }
            } finally {
              setDeleting(false);
            }
          }}
          headerSlot={
            <div className={styles.sidebarHeader}>
              <div className={styles.sidebarTabs}>
                <button
                  className={`${styles.sidebarTab} ${sidebarTab === "content" ? styles.sidebarTabActive : ""}`}
                  onClick={() => setSidebarTab("content")}
                >
                  Контент
                </button>
                <button
                  className={`${styles.sidebarTab} ${sidebarTab === "css" ? styles.sidebarTabActive : ""}`}
                  onClick={() => setSidebarTab("css")}
                >
                  Стили
                </button>
              </div>

              {loadingTree ? (
                <div className={styles.sidebarLoadingInline}>
                  <span className={styles.inlineSpinner} /> Загрузка…
                </div>
              ) : sidebarTab === "content" && (
                <div className={styles.sidebarActions}>
                  <button className={styles.sidebarTab} onClick={addPage} disabled={creatingPage || creatingFolder || loadingTree}>
                    {creatingPage ? "Создаём…" : "+ Страница"}
                  </button>
                  <button className={styles.sidebarTab} onClick={addFolder} disabled={creatingFolder || creatingPage || loadingTree}>
                    {creatingFolder ? "Создаём…" : "+ Папка"}
                  </button>

                </div>
              )}  
            </div>
          }
          contentSlot={
            sidebarTab === "css" ? (
              <div className={styles.cssPanel}>
                  <div className={styles.cssGrid}>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Основной</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          className={styles.colorInput}
                          type="color"
                          value={effectiveCss.primary}
                        onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), primary: e.target.value }))}
                      />
                      <span className={styles.cssValue}>{effectiveCss.primary}</span>
                      </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Фон</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          className={styles.colorInput}
                          type="color"
                        value={effectiveCss.background}
                        onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), background: e.target.value }))}
                      />
                      <span className={styles.cssValue}>{effectiveCss.background}</span>
                      </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Поверхность</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          className={styles.colorInput}
                        type="color"
                        value={effectiveCss.surface}
                        onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), surface: e.target.value }))}
                      />
                      <span className={styles.cssValue}>{effectiveCss.surface}</span>
                    </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Текст</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          className={styles.colorInput}
                        type="color"
                value={effectiveCss.text}
                onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), text: e.target.value }))}
              />
              <span className={styles.cssValue}>{effectiveCss.text}</span>
            </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Границы</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          className={styles.colorInput}
                type="color"
                value={effectiveCss.border}
                onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), border: e.target.value }))}
              />
              <span className={styles.cssValue}>{effectiveCss.border}</span>
            </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Сайдбар</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          className={styles.colorInput}
                type="color"
                value={effectiveCss.sidebar}
                onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), sidebar: e.target.value }))}
              />
              <span className={styles.cssValue}>{effectiveCss.sidebar}</span>
            </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Шрифт</div>
                      <div className={styles.cssFieldControl}>
                        <select
                          value={effectiveCss.fontFamily}
                          onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), fontFamily: e.target.value }))}
                      >
                        <option value="Inter, system-ui, -apple-system, sans-serif">Inter</option>
                        <option value="Roboto, system-ui, -apple-system, sans-serif">Roboto</option>
                        <option value={'"Helvetica Neue", Arial, sans-serif'}>Helvetica Neue</option>
                        <option value="Georgia, serif">Georgia</option>
                        <option value={'"Times New Roman", serif'}>Times New Roman</option>
                        <option value={'"Source Serif Pro", Georgia, serif'}>Source Serif Pro</option>
                      </select>
                    </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Размер шрифта</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          type="range"
                        min={14}
                        max={22}
                        value={effectiveCss.fontSize}
                        onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), fontSize: Number(e.target.value) }))}
                      />
                      <span className={styles.cssValue}>{effectiveCss.fontSize}px</span>
                    </div>
                    </div>
                    <div className={styles.cssFieldRow}>
                      <div className={styles.cssFieldLabel}>Радиус</div>
                      <div className={styles.cssFieldControl}>
                        <input
                          type="range"
                        min={0}
                        max={24}
                        value={effectiveCss.radius}
                        onChange={(e) => setCssSettings((prev) => ({ ...(prev || resolveInitialCssSettings()), radius: Number(e.target.value) }))}
                      />
                      <span className={styles.cssValue}>{effectiveCss.radius}px</span>
                    </div>
                  </div>
                </div>
                <div className={styles.cssSaveCard}>
                  <div className={styles.cssSaveRow}>
                    <div className={`${styles.pill} ${styles[`pill_${cssStatus === "error" ? "error" : cssSaving ? "saving" : "saved"}`]}`}>
                      <span className={styles.pillIcon}>
                        {cssSaving ? <span className={styles.pillSpinner} /> : cssStatus === "error" ? "✖" : "✓"}
                      </span>
                    </div>
                    <button className={styles.buttonSave} onClick={() => saveCssSettings()} disabled={cssSaving}>
                      {cssSaving ? "Сохраняем…" : "Сохранить стили"}
                    </button>
                    <button
                      className={styles.buttonReset}
                      onClick={async () => {
                        const ok = confirm("Сбросить стили к стандартным?");
                        if (!ok) return;
                        const next = { ...DEFAULT_CSS_SETTINGS };
                        setCssSettings(next);
                        setCssStatus("idle");
                        applyDesignTokens(next);
                        await saveCssSettings(next);
                      }}
                    >
                      Сбросить
                    </button>
                  </div>
                </div>
              </div>
            ) : undefined
          }
        />
      </div>

      <main className={styles.main}>
        <div className={styles.toolbar}>
          <div className={styles.toolbarLeft}>
            <div className={styles.toggle}>
              <button className={mode === "markdown" ? styles.active : ""} onClick={() => setMode("markdown")}>
                Редактор
              </button>
              <button className={mode === "preview" ? styles.active : ""} onClick={() => setMode("preview")}>
                Просмотр
              </button>
            </div>
          </div>
          <div className={styles.toolbarRight}>
            <button
              type="button"
              className={`${styles.pill} ${styles.pillButton} ${styles[`pill_${statusInfo.state}`]}`}
              onClick={triggerSaveNow}
              disabled={saving}
              aria-label="Сохранить"
              title="Сохранить"
            >
              <span className={styles.pillIcon}>
                {statusInfo.state === "saving" ? (
                  <span className={styles.pillSpinner} />
                ) : statusInfo.state === "error" ? (
                  "✖"
                ) : (
                  statusInfo.state === "saved" ? "✓" : "✖"
                )}
              </span>
              <div className={styles.pillText}>
                <div className={styles.pillSubtext}>
                  {statusInfo.time ? formatTime(statusInfo.time) : fallbackTime ? formatTime(fallbackTime) : "--:--:--"}
                </div>
              </div>
            </button>
            <button className={styles.buttonOpen} type="button" onClick={openPublished} title="Открыть опубликованную версию">
              <OpenInNewRoundedIcon fontSize="small" />
            </button>
          </div>
        </div>

        <div className={styles.formShell}>
          {topError && (
            <div className={styles.statusError} role="status" aria-live="polite">
              {topError}
            </div>
          )}

          <div className={styles.panel}>
          <div className={styles.panelActions}>
            <div className={styles.toggleRow}>
              <span>Главная страница</span>
              <label className={styles.switch}>
                <input
                  type="checkbox"
                  checked={Boolean(frontmatter.home)}
                  onChange={(e) => onFieldChange("home", e.target.checked)}
                />
                <span className={styles.slider} />
              </label>
            </div>
            <button
              className={styles.deleteLink}
              onClick={deletePage}
              disabled={deleting || saving || loading || !activePath}
            >
              {deleting ? "Удаление…" : "Удалить"}
            </button>
          </div>
          <div className={styles.grid}>
            <div className={styles.field}>
              <label>Заголовок</label>
              <input
                value={frontmatter.title}
                onChange={(e) => onFieldChange("title", e.target.value)}
                placeholder="Заголовок"
              />
            </div>
            <div className={styles.field}>
              <label>Адрес</label>
              <input
                value={frontmatter.slug}
                onChange={(e) => onFieldChange("slug", e.target.value)}
                placeholder="адрес"
              />
            </div>
          </div>
          <div className={styles.grid}>
            <div className={styles.field}>
              <label>Папка</label>
              <select value={selectedFolder} onChange={(e) => setSelectedFolder(e.target.value)}>
                <option value="">/ (корень)</option>
                {folderOptions.map((folder) => (
                  <option key={folder} value={folder}>
                    {folder}
                  </option>
                ))}
              </select>
            </div>
            <div className={styles.field}>
              <label>Черновик</label>
              <div className={styles.toggleRow}>
                <label className={styles.switch}>
                  <input
                    type="checkbox"
                    checked={frontmatter.draft !== false}
                    onChange={(e) => onFieldChange("draft", e.target.checked)}
                  />
                  <span className={styles.slider} />
                </label>
              </div>
            </div>
          </div>
          </div>
        </div>

        <div className={styles.split}>
          <div className={styles.contentShell}>
            {mode === "markdown" ? (
                <div className={styles.editorArea} ref={editorAreaRef}>
                <div className={`${styles.composerHeader} ${styles.composerHeaderHidden}`}>
                  <div className={styles.editorToolbar}>
                    <div className={styles.toolbarGroup}>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("undo")} type="button" title="Отменить">
                        <UndoRoundedIcon fontSize="small" />
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("redo")} type="button" title="Повторить">
                        <RedoRoundedIcon fontSize="small" />
                      </button>
                    </div>
                    <div className={styles.toolbarGroup}>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("h1")} type="button" title="Заголовок 1">
                        H1
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("h2")} type="button" title="Заголовок 2">
                        H2
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("h3")} type="button" title="Заголовок 3">
                        H3
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("h4")} type="button" title="Заголовок 4">
                        H4
                      </button>
                    </div>
                    <div className={styles.toolbarGroup}>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("bold")} type="button" title="Полужирный">
                        <FormatBoldRoundedIcon fontSize="small" />
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("italic")} type="button" title="Курсив">
                        <FormatItalicRoundedIcon fontSize="small" />
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("code")} type="button" title="Код">
                        <CodeRoundedIcon fontSize="small" />
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("link")} type="button" title="Ссылка">
                        <InsertLinkRoundedIcon fontSize="small" />
                      </button>
                    </div>
                    <div className={styles.toolbarGroup}>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("bullet")} type="button" title="Маркированный список">
                        <FormatListBulletedRoundedIcon fontSize="small" />
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("ordered")} type="button" title="Нумерованный список">
                        <FormatListNumberedRoundedIcon fontSize="small" />
                      </button>
                      <button className={styles.toolbarButton} onClick={() => handleToolbarAction("quote")} type="button" title="Цитата">
                        <FormatQuoteRoundedIcon fontSize="small" />
                      </button>
                    </div>
                    <div className={styles.toolbarGroup}>
                      <button
                        className={styles.toolbarButton}
                        onClick={handleImagePicker}
                        type="button"
                        title="Добавить изображение из файла"
                        disabled={imageInserting}
                      >
                        {imageInserting ? "Загрузка…" : <InsertPhotoRoundedIcon fontSize="small" />}
                      </button>
                    </div>
                  </div>
                </div>
                <div className={styles.editorSurface} ref={editorSurfaceRef}>
                  {plusCoords && mode === "markdown" && (
                    <div
                      className={`${styles.plusButtonWrap} ${plusOpen ? styles.plusOpen : ""}`}
                      ref={plusMenuRef}
                      style={{ left: plusCoords.left, top: plusCoords.top }}
                    >
                      <button
                        type="button"
                        className={styles.plusButton}
                        onClick={() => setPlusOpen((prev) => !prev)}
                        aria-label="Добавить блок"
                      >
                        +
                      </button>
                      {plusOpen && (
                        <div className={styles.plusMenu}>
                          <button type="button" onClick={() => { handleToolbarAction("h2"); setPlusOpen(false); }}>
                            Заголовок 2
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("h3"); setPlusOpen(false); }}>
                            Заголовок 3
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("text"); setPlusOpen(false); }}>
                            Текст
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("remove"); setPlusOpen(false); }}>
                            Удалить блок
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("bullet"); setPlusOpen(false); }}>
                            Маркированный список
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("ordered"); setPlusOpen(false); }}>
                            Нумерованный список
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("quote"); setPlusOpen(false); }}>
                            Цитата
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("link"); setPlusOpen(false); }}>
                            Ссылка
                          </button>
                          <button type="button" onClick={() => { handleToolbarAction("code"); setPlusOpen(false); }}>
                            Код
                          </button>
                          <button type="button" onClick={() => { handleImagePicker(); setPlusOpen(false); }}>
                            Изображение
                          </button>
                        </div>
                      )}
                    </div>
                  )}
                  <div ref={editorParentRef} className={styles.cmHost} />
                </div>
                {showBubble && bubbleCoords && (
                  <div
                    ref={bubbleRef}
                    className={styles.bubbleMenu}
                    style={{ left: bubbleCoords.left, top: bubbleCoords.top }}
                  >
                    <button className={styles.bubbleButton} onClick={() => handleToolbarAction("bold")} type="button">
                      B
                    </button>
                    <button className={styles.bubbleButton} onClick={() => handleToolbarAction("italic")} type="button">
                      I
                    </button>
                    <button className={styles.bubbleButton} onClick={() => handleToolbarAction("code")} type="button">
                      {'</>'}
                    </button>
                    <button className={styles.bubbleButton} onClick={() => handleToolbarAction("link")} type="button">
                      Ссылка
                    </button>
                  </div>
                )}
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  className={styles.hiddenFileInput}
                  onChange={handleFileInputChange}
                />
              </div>
            ) : (
              <div className={`${styles.previewArea}`}>
                <Breadcrumbs crumbs={[{ title: frontmatter.title || "Без названия", href: null }]} />
                {previewLoading ? (
                  <div>Создание предпросмотра…</div>
                ) : (
                  <article className="prose" dangerouslySetInnerHTML={{ __html: previewHtml }} onClick={handlePreviewClick} />
                )}
              </div>
            )}
            {loading && (
              <div className={styles.loadingOverlay}>
                <div className={styles.loadingMessage}>
                  <span className={styles.inlineSpinner} /> Загрузка страницы…
                </div>
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}

function formatTime(date: Date) {
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

class ListMarkerWidget extends WidgetType {
  constructor(private label: string, private indent: number) {
    super();
  }

  eq(other: WidgetType) {
    return other instanceof ListMarkerWidget && other.label === this.label && other.indent === this.indent;
  }

  toDOM() {
    const span = document.createElement("span");
    span.className = "cm-list-marker";
    if (this.indent > 0) {
      span.style.marginLeft = `${this.indent}ch`;
    }
    span.textContent = this.label;
    return span;
  }
}

class InlineCodeWidget extends WidgetType {
  constructor(private text: string) {
    super();
  }

  eq(other: WidgetType) {
    return other instanceof InlineCodeWidget && other.text === this.text;
  }

  toDOM() {
    const code = document.createElement("code");
    code.className = "cm-inline-code";
    code.textContent = this.text;
    return code;
  }
}

const markdownLineDecorations = ViewPlugin.fromClass(
  class {
    decorations;
    constructor(view: EditorView) {
      this.decorations = buildMarkdownDecorations(view);
    }
    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged) {
        this.decorations = buildMarkdownDecorations(update.view);
      }
    }
  },
  {
    decorations: (v) => v.decorations,
  },
);

function buildMarkdownDecorations(view: EditorView) {
  const builder = new RangeSetBuilder<Decoration>();
  const doc = view.state.doc;
  let inFence = false;
  for (let i = 1; i <= doc.lines; i++) {
    const line = doc.line(i);
    const text = line.text;
    const trimmed = text.trim();

    if (/^```/.test(trimmed)) {
      const closesSameLine = trimmed.slice(3).includes("```");
      const classes = ["cm-md-codeblock", "cm-md-codeFence"];
      if (!inFence) classes.push("cm-md-codeblock-start");
      if (inFence || closesSameLine) classes.push("cm-md-codeblock-end");
      builder.add(line.from, line.from, Decoration.line({ class: classes.join(" ") }));
      if (!closesSameLine) inFence = !inFence;
      else inFence = false;
      continue;
    }

    if (inFence) {
      builder.add(line.from, line.from, Decoration.line({ class: "cm-md-codeblock cm-md-codeblock-body" }));
      continue;
    }

    if (!trimmed) continue;

    const lineClasses: string[] = [];

    const headingMatch = trimmed.match(/^(#{1,6})\s+/);
    if (headingMatch) {
      const level = Math.min(headingMatch[1].length, 3);
      lineClasses.push(`cm-md-h${level}`);
    }

    if (/^>/.test(trimmed)) {
      lineClasses.push("cm-md-quote");
    }

    const listMatch = text.match(/^(\s*)(\*|\+|-)\s+/);
    if (listMatch) {
      const indent = listMatch[1].length;
      const prefixLength = listMatch[0].length;
      lineClasses.push("cm-md-list");
      builder.add(line.from, line.from + prefixLength, Decoration.replace({ widget: new ListMarkerWidget("•", indent) }));
    }

    const orderedMatch = text.match(/^(\s*)(\d+)\.\s+/);
    if (orderedMatch) {
      const indent = orderedMatch[1].length;
      const num = orderedMatch[2];
      const prefixLength = orderedMatch[0].length;
      lineClasses.push("cm-md-list-ol");
      builder.add(line.from, line.from + prefixLength, Decoration.replace({ widget: new ListMarkerWidget(`${num}.`, indent) }));
    }

    if (lineClasses.length === 0) {
      lineClasses.push("cm-md-paragraph");
    }

    // Line-level decoration first to maintain builder ordering
    builder.add(line.from, line.from, Decoration.line({ class: lineClasses.join(" ") }));

    const codeRe = /`([^`]+)`/g;
    let match: RegExpExecArray | null;
    while ((match = codeRe.exec(text)) !== null) {
      if (match[0].length < 2) continue;
      const start = line.from + match.index;
      const end = start + match[0].length;
      builder.add(start, end, Decoration.replace({ widget: new InlineCodeWidget(match[1]) }));
    }
  }
  return builder.finish();
}

const imageDataUrlPreviewPlugin = ViewPlugin.fromClass(
  class {
    decorations;
    constructor(view: EditorView) {
      this.decorations = buildImageWidgets(view);
    }
    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged) {
        this.decorations = buildImageWidgets(update.view);
      }
    }
  },
  {
    decorations: (v) => v.decorations,
  },
);

function buildImageWidgets(view: EditorView) {
  const builder = new RangeSetBuilder<Decoration>();
  const docText = view.state.doc.toString();
  const patterns = [
    /!\[(.*?)\]\(([^)]+)\)/g, // standard markdown
    /!\[\[([^\]]+)\]\]/g, // obsidian wikilink
  ];
  for (const re of patterns) {
    let match: RegExpExecArray | null;
    while ((match = re.exec(docText))) {
      const start = match.index;
      const end = start + match[0].length;
      const rawSrc = re === patterns[0] ? match[2] : match[1];
      const alt = re === patterns[0] ? match[1] : match[1];
      const resolved = resolveImageUrl(rawSrc, notePathForWidgets);
      builder.add(
        start,
        end,
        Decoration.replace({
          widget: new ImageWidget(resolved, alt, rawSrc),
          inclusive: true,
        }),
      );
    }
  }
  return builder.finish();
}

class ImageWidget extends WidgetType {
  constructor(private src: string, private alt: string, private raw: string) {
    super();
  }
  eq(other: ImageWidget) {
    return this.src === other.src && this.alt === other.alt && this.raw === other.raw;
  }
  toDOM(view: EditorView) {
    const wrapper = document.createElement("div");
    wrapper.className = styles.cmImageWidget;
    const loader = document.createElement("div");
    loader.className = styles.cmImageLoader;
    const spinner = document.createElement("span");
    spinner.className = styles.cmImageSpinner;
    loader.appendChild(spinner);
    wrapper.appendChild(loader);
    const img = document.createElement("img");
    img.src = this.src;
    img.alt = this.alt || "image";
    img.loading = "lazy";
    img.addEventListener("load", () => {
      loader.style.display = "none";
    });
    img.addEventListener("error", () => {
      loader.style.display = "none";
    });
    wrapper.appendChild(img);
    const caption = document.createElement("span");
    caption.className = styles.cmImageCaption;
    caption.textContent = this.alt || "image";
    wrapper.appendChild(caption);

    const actions = document.createElement("div");
    actions.className = styles.cmImageActions;
    const deleteBtn = document.createElement("button");
    deleteBtn.type = "button";
    deleteBtn.className = styles.cmImageDelete;
    deleteBtn.textContent = "Удалить";
    deleteBtn.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      deleteBtn.disabled = true;
      wrapper.classList.add(styles.cmImageDeleting);
      removeImageFromDoc(view, this.raw, this.alt);
    });
    actions.appendChild(deleteBtn);
    wrapper.appendChild(actions);

    wrapper.addEventListener("mousedown", (e) => {
      e.preventDefault();
      selectImageRange(view, this.src, this.alt);
    });

    return wrapper;
  }
}

function resolveImageUrl(rawSrc: string, notePath?: string | null) {
  const src = rawSrc.trim();
  if (!src) return "";
  if (/^data:/i.test(src)) return src;
  if (/^https?:\/\//i.test(src)) return src;
  if (src.startsWith("/api/editor/assets/")) return src;

  const cleaned = src.replace(/^\.\//, "").replace(/^\/+/, "");
  const folder = notePath ? getFolderFromPath(notePath) : "";
  const candidates: string[] = [];
  if (folder) candidates.push(`${folder}/${cleaned}`);
  candidates.push(`.np-assets/${cleaned}`);
  candidates.push(cleaned);

  const picked = candidates.find(Boolean) || cleaned;
  return `/api/editor/assets/${picked.replace(/^\/+/, "")}`;
}

function findImageRange(state: EditorState, src: string, alt: string) {
  const docText = state.doc.toString();
  const needle = `![${alt || ""}](${src})`;
  const start = docText.indexOf(needle);
  if (start === -1) return null;
  return { from: start, to: start + needle.length };
}

function selectImageRange(view: EditorView, src: string, alt: string) {
  const range = findImageRange(view.state, src, alt);
  if (!range) return;
  view.dispatch({ selection: { anchor: range.from, head: range.to } });
  view.focus();
}

function removeImageFromDoc(view: EditorView, src: string, alt: string) {
  const range = findImageRange(view.state, src, alt);
  if (!range) return;
  view.dispatch({ changes: { from: range.from, to: range.to, insert: "" } });
  view.focus();
  void deleteRemoteAsset(src);
}

async function deleteRemoteAsset(src: string) {
  const isAsset = src.includes(".np-assets") || src.includes("/api/editor/assets/");
  if (!isAsset) return;
  try {
    await fetch("/api/editor/asset/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url: src, force: true }),
    });
  } catch {
    // ignore failures; the markdown reference is already removed
  }
}

function wrapSelectionWith(view: EditorView, before: string, after: string, placeholder: string) {
  const { state } = view;
  const tr = state.changeByRange((range) => {
    const selected = state.sliceDoc(range.from, range.to) || placeholder;
    const insert = `${before}${selected}${after}`;
    const start = range.from + before.length;
    const end = start + selected.length;
    return {
      changes: { from: range.from, to: range.to, insert },
      range: EditorSelection.range(start, end),
    };
  });
  view.dispatch(tr);
}

function toggleLinePrefix(view: EditorView, marker: string) {
  const { state } = view;
  const tr = state.changeByRange((range) => {
    const line = state.doc.lineAt(range.from);
    const hasPrefix = state.sliceDoc(line.from, line.from + marker.length) === marker;
    const change = hasPrefix
      ? { from: line.from, to: line.from + marker.length, insert: "" }
      : { from: line.from, insert: marker };
    const delta = hasPrefix ? -marker.length : marker.length;
    const newFrom = Math.max(line.from, range.from + delta);
    const newTo = Math.max(line.from, range.to + delta);
    return {
      changes: change,
      range: EditorSelection.range(newFrom, newTo),
    };
  });
  view.dispatch(tr);
}

function applyBlockPrefix(view: EditorView, type: "h1" | "h2" | "h3" | "h4" | "text" | "bullet" | "ordered" | "quote") {
  const markerMap: Record<typeof type, string> = {
    h1: "# ",
    h2: "## ",
    h3: "### ",
    h4: "#### ",
    text: "",
    bullet: "- ",
    ordered: "1. ",
    quote: "> ",
  };
  const markers = Object.values(markerMap).filter(Boolean);
  const target = markerMap[type];

  const tr = view.state.changeByRange((range) => {
    const line = view.state.doc.lineAt(range.from);
    let newText = line.text;

    // Remove any existing known marker
    for (const m of markers) {
      if (newText.startsWith(m)) {
        newText = newText.slice(m.length);
        break;
      }
    }

    const insert = `${target}${newText}`;
    const from = line.from;
    const to = line.to;
    const delta = insert.length - (line.to - line.from);
    const newFrom = Math.max(line.from, range.from + delta);
    const newTo = Math.max(line.from, range.to + delta);

    return {
      changes: { from, to, insert },
      range: EditorSelection.range(newFrom, newTo),
    };
  });

  view.dispatch(tr);
}

function removeLine(view: EditorView) {
  const { doc, selection } = view.state;
  const pos = selection.main.head;
  const line = doc.lineAt(pos);
  let from = line.from;
  let to = line.to;

  // Prefer removing the delimiter after the line to keep preceding content intact.
  if (to < doc.length && doc.sliceString(to, to + 1) === "\n") {
    to += 1;
  } else if (from > 0 && doc.sliceString(from - 1, from) === "\n") {
    from -= 1;
  }

  const removedLength = to - from;
  const nextLength = doc.length - removedLength;
  const newPos = Math.min(from, Math.max(0, nextLength));

  view.dispatch({
    changes: { from, to, insert: "" },
    selection: EditorSelection.cursor(newPos),
  });
}

function insertLinkMarkdown(view: EditorView, url: string, textFallback = "ссылка") {
  const { state } = view;
  const tr = state.changeByRange((range) => {
    const selected = state.sliceDoc(range.from, range.to) || textFallback;
    const insert = `[${selected}](${url})`;
    const pos = range.from + insert.length;
    return {
      changes: { from: range.from, to: range.to, insert },
      range: EditorSelection.cursor(pos),
    };
  });
  view.dispatch(tr);
}

function insertImageMarkdown(view: EditorView, url: string, altFallback = "image") {
  const { state } = view;
  const tr = state.changeByRange((range) => {
    const altText = state.sliceDoc(range.from, range.to) || altFallback;
    const normalized = url.startsWith("/") || url.startsWith(".") || url.startsWith("http") ? url : `./${url}`;
    // Ensure a blank line before/after so the image is a standalone block (but cap to a single blank line).
    const before = state.doc.sliceString(0, range.from);
    const after = state.doc.sliceString(range.to);
    const beforeNewlines = (before.match(/\n+$/) || [""])[0].length;
    const afterNewlines = (after.match(/^\n+/) || [""])[0].length;
    const leading = beforeNewlines >= 2 ? "" : "\n".repeat(2 - beforeNewlines);
    const trailing = afterNewlines >= 2 ? "" : "\n".repeat(2 - afterNewlines);
    const insert = `${leading}![${altText}](${normalized})${trailing}`;
    const pos = range.from + insert.length;
    return {
      changes: { from: range.from, to: range.to, insert },
      range: EditorSelection.cursor(pos),
    };
  });
  view.dispatch(tr);
}

function ensureRootFilename(input: string) {
  const base = input.trim() || "без-названия";
  const withoutSlashes = base.replace(/[\\/]+/g, "-");
  const trimmed = withoutSlashes.replace(/^\.+/, "");
  const name = trimmed || "без-названия";
  return name.endsWith(".md") ? name : `${name}.md`;
}

function normalizeFolderInput(input: string) {
  const trimmed = input.replace(/^[\\/]+|[\\/]+$/g, "").trim();
  if (!trimmed || trimmed.includes("..")) return "";
  return trimmed;
}

function normalizePath(input?: string | null) {
  if (!input) return "";
  const trimmed = input.replace(/^\/+/, "");
  if (!trimmed) return "";
  return trimmed.endsWith(".md") ? trimmed : `${trimmed}.md`;
}

function generateDefaultPageIdentifiers(notes: NoteRecord[], folder: string) {
  const baseTitle = "Страница";
  const baseSlug = "page";
  const existingPaths = new Set(notes.map((n) => normalizePath(n.path)));
  const existingSlugs = new Set(notes.map((n) => (n.slug || "").trim()));
  let counter = 1;
  while (true) {
    const suffix = counter === 1 ? "" : `-${counter}`;
    const title = `${baseTitle}${suffix}`;
    const slug = `${baseSlug}${suffix}`;
    const path = buildPath(title, folder);
    if (!existingPaths.has(normalizePath(path)) && !existingSlugs.has(slug)) {
      return { title, slug, path };
    }
    counter += 1;
  }
}

function generateDefaultFolderName(folders: string[], extra: Set<string>) {
  const base = "Раздел";
  const existing = new Set<string>();
  folders.forEach((f) => existing.add(f.trim()));
  extra.forEach((f) => existing.add(f.trim()));
  let counter = 1;
  while (true) {
    const suffix = counter === 1 ? "" : `-${counter}`;
    const name = `${base}${suffix}`;
    const normalized = normalizeFolderInput(name);
    if (normalized && !existing.has(normalized)) {
      return normalized;
    }
    counter += 1;
  }
}

function buildPath(title: string, folder: string) {
  const file = ensureRootFilename(title);
  if (!folder) return file;
  return `${folder}/${file}`;
}

function getFolderFromPath(path: string) {
  const parts = path.split("/").filter(Boolean);
  if (parts.length <= 1) return "";
  return parts.slice(0, -1).join("/");
}

function buildCategories(notes: NoteRecord[], folders: Set<string>) {
  const map = new Map<string, { name: string; slug: string; notes: { title: string; slug: string; isDraft?: boolean; isHome?: boolean }[] }>();
  for (const note of notes) {
    const name = note.category?.trim();
    if (!name) continue;
    const key = name.toLowerCase();
    if (!map.has(key)) {
      map.set(key, { name, slug: slugifySegment(name) || name, notes: [] });
    }
    map.get(key)!.notes.push({ title: note.title, slug: note.slug, isDraft: note.draft, isHome: note.home });
  }
  folders.forEach((folder) => {
    const name = folder.trim();
    if (!name) return;
    if (name === ".keep") return;
    const key = name.toLowerCase();
    if (!map.has(key)) {
      map.set(key, { name, slug: slugifySegment(name) || name, notes: [] });
    }
  });
  return [...map.values()];
}

function deriveDesignTokens(settings: CssSettings) {
  const primary = settings.primary;
  const primaryHover = adjustLightness(primary, -10);
  const primaryActive = adjustLightness(primary, -16);
  const primaryDisabled = adjustLightness(primary, 18, 0.5);
  const surfaceMuted = adjustLightness(settings.surface, -4);
  const surfaceHover = adjustSurfaceHover(settings.surface);
  const surfaceMutedHover = adjustSurfaceHover(surfaceMuted);
  return {
    ...settings,
    primary,
    primaryHover,
    primaryActive,
    primaryDisabled,
    surfaceMuted,
    surfaceHover,
    surfaceMutedHover,
    borderStrong: adjustLightness(settings.border, -10),
  };
}

function applyDesignTokens(settings: CssSettings) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  const tokens = deriveDesignTokens(settings);
  root.style.setProperty("--design-primary", tokens.primary);
  root.style.setProperty("--design-primary-hover", tokens.primaryHover);
  root.style.setProperty("--design-primary-active", tokens.primaryActive);
  root.style.setProperty("--design-primary-disabled", tokens.primaryDisabled);
  root.style.setProperty("--design-bg", tokens.background);
  root.style.setProperty("--design-surface", tokens.surface);
  root.style.setProperty("--design-surface-muted", tokens.surfaceMuted);
  root.style.setProperty("--design-surface-hover", tokens.surfaceHover);
  root.style.setProperty("--design-surface-muted-hover", tokens.surfaceMutedHover);
  root.style.setProperty("--design-sidebar", tokens.sidebar);
  root.style.setProperty("--design-border", tokens.border);
  root.style.setProperty("--design-text", tokens.text);
  root.style.setProperty("--design-radius", `${tokens.radius}px`);
  root.style.setProperty("--design-font", tokens.fontFamily);
  root.style.setProperty("--design-font-size", `${tokens.fontSize}px`);
  root.style.setProperty("--design-shadow", tokens.shadow ? "0 8px 20px rgba(0,0,0,0.08)" : "none");
  root.style.setProperty("--radius-md", `${tokens.radius}px`);
  root.style.setProperty("--radius-lg", `${Math.max(tokens.radius, 14)}px`);
  root.style.setProperty("--radius-sm", `${Math.max(Math.round(tokens.radius * 0.65), 6)}px`);
  root.style.setProperty("--radius", `${tokens.radius}px`);
  root.style.setProperty("--font-size-base", `${tokens.fontSize}px`);
  root.style.fontSize = `${tokens.fontSize}px`;
  root.style.setProperty("--font-body", tokens.fontFamily);
  root.style.setProperty("--font-heading", tokens.fontFamily);
  root.style.setProperty("--font", tokens.fontFamily);

  // Keep legacy theme variables aligned to the primary selections
  root.style.setProperty("--color-accent", tokens.primary);
  root.style.setProperty("--color-accent-strong", tokens.primaryHover);
  root.style.setProperty("--color-accent-stronger", tokens.primaryActive);
  root.style.setProperty("--color-bg", tokens.background);
  root.style.setProperty("--color-surface", tokens.surface);
  root.style.setProperty("--color-surface-muted", tokens.surfaceMuted);
  root.style.setProperty("--color-surface-hover", tokens.surfaceHover);
  root.style.setProperty("--color-sidebar", tokens.sidebar);
  root.style.setProperty("--color-border", tokens.border);
  root.style.setProperty("--color-text", tokens.text);
}

function adjustLightness(color: string, delta: number, opacity = 1) {
  const { h, s, l } = hexToHsl(color || "#000000");
  const nextL = clamp(l + delta, 0, 100);
  return hslToHex(h, s, nextL, opacity);
}

function hexToHsl(hex: string) {
  let sanitized = hex.trim();
  if (sanitized.startsWith("#")) sanitized = sanitized.slice(1);
  if (sanitized.length === 3) {
    sanitized = sanitized
      .split("")
      .map((c) => c + c)
      .join("");
  }
  const num = parseInt(sanitized || "000000", 16);
  const r = (num >> 16) & 255;
  const g = (num >> 8) & 255;
  const b = num & 255;

  const rNorm = r / 255;
  const gNorm = g / 255;
  const bNorm = b / 255;
  const max = Math.max(rNorm, gNorm, bNorm);
  const min = Math.min(rNorm, gNorm, bNorm);
  let h = 0;
  let s = 0;
  const l = (max + min) / 2;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case rNorm:
        h = (gNorm - bNorm) / d + (gNorm < bNorm ? 6 : 0);
        break;
      case gNorm:
        h = (bNorm - rNorm) / d + 2;
        break;
      case bNorm:
        h = (rNorm - gNorm) / d + 4;
        break;
    }
    h /= 6;
  }

  return { h: h * 360, s: s * 100, l: l * 100 };
}

function hslToHex(h: number, s: number, l: number, opacity = 1) {
  const sNorm = s / 100;
  const lNorm = l / 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = sNorm * Math.min(lNorm, 1 - lNorm);
  const f = (n: number) => lNorm - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  const r = Math.round(255 * f(0));
  const g = Math.round(255 * f(8));
  const b = Math.round(255 * f(4));
  const toHex = (x: number) => x.toString(16).padStart(2, "0");
  const hex = `#${toHex(r)}${toHex(g)}${toHex(b)}`;
  if (opacity >= 1) return hex;
  const alpha = toHex(Math.round(clamp(opacity, 0, 1) * 255));
  return `${hex}${alpha}`;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function adjustSurfaceHover(color: string) {
  const { l } = hexToHsl(color || "#ffffff");
  const delta = l > 50 ? -6 : 6;
  return adjustLightness(color || "#ffffff", delta);
}

function resolveInitialCssSettings(): CssSettings {
  if (typeof window === "undefined") return DEFAULT_CSS_SETTINGS;
  const root = getComputedStyle(document.documentElement);
  const pick = (name: string, fallback: string) => root.getPropertyValue(name).trim() || fallback;
  const fontSizeRaw = root.getPropertyValue("--font-size-base").trim();
  const fontSize = Number(parseFloat(fontSizeRaw)) || DEFAULT_CSS_SETTINGS.fontSize;
  return {
    primary: pick("--color-accent", DEFAULT_CSS_SETTINGS.primary),
    background: pick("--color-bg", DEFAULT_CSS_SETTINGS.background),
    surface: pick("--color-surface", DEFAULT_CSS_SETTINGS.surface),
    text: pick("--color-text", DEFAULT_CSS_SETTINGS.text),
    border: pick("--color-border", DEFAULT_CSS_SETTINGS.border),
    sidebar: pick("--color-sidebar", DEFAULT_CSS_SETTINGS.sidebar),
    fontFamily: pick("--font-body", DEFAULT_CSS_SETTINGS.fontFamily),
    fontSize,
    radius: DEFAULT_CSS_SETTINGS.radius,
    shadow: DEFAULT_CSS_SETTINGS.shadow,
  };
}
