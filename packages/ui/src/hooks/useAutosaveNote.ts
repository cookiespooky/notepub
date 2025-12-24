import { useCallback, useEffect, useRef, useState } from "react";

export type FrontmatterShape = Record<string, unknown> & {
  title?: string;
  slug?: string;
  category?: string;
  home?: boolean;
  draft?: boolean;
};

export type NoteDraft<TFrontmatter extends FrontmatterShape = FrontmatterShape> = {
  path: string;
  frontmatter: TFrontmatter;
  body: string;
};

type ValidationResult = { ok: true } | { ok: false; error?: string };

export type SaveFn<TFrontmatter extends FrontmatterShape = FrontmatterShape> = (
  input: NoteDraft<TFrontmatter>,
) => Promise<{ path: string; slug?: string }>;

type UseAutosaveOptions<TFrontmatter extends FrontmatterShape = FrontmatterShape> = {
  /** Initial note state. When path changes, internal state resets to this value. */
  note: NoteDraft<TFrontmatter>;
  /** Validation gate (e.g., title/slug required, slug uniqueness). */
  validate: (note: NoteDraft<TFrontmatter>) => ValidationResult;
  /** Persist current note as-is (draft flag respected). */
  save: SaveFn<TFrontmatter>;
  /** Debounce delay in ms. Default: 3000. */
  delayMs?: number;
  /** Pause autosave while a publish is running. */
  publishInFlight?: boolean;
};

export function useAutosaveNote<TFrontmatter extends FrontmatterShape = FrontmatterShape>({
  note: initialNote,
  validate,
  save,
  delayMs = 3000,
  publishInFlight = false,
}: UseAutosaveOptions<TFrontmatter>) {
  const [note, setNote] = useState<NoteDraft<TFrontmatter>>(initialNote);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null);
  const noteRef = useRef(note);
  const publishRef = useRef(publishInFlight);
  publishRef.current = publishInFlight;

  // Reset state when a different file is loaded.
  useEffect(() => {
    if (initialNote.path !== noteRef.current.path) {
      noteRef.current = initialNote;
      setNote(initialNote);
      setDirty(false);
      setError(null);
      setSaving(false);
    }
  }, [initialNote]);

  // Track note changes and mark dirty.
  const updateNote = useCallback(
    (updater: NoteDraft<TFrontmatter> | ((prev: NoteDraft<TFrontmatter>) => NoteDraft<TFrontmatter>)) => {
      setNote((prev) => {
        const next = typeof updater === "function" ? (updater as any)(prev) : updater;
        noteRef.current = next;
        setDirty(true);
        return next;
      });
    },
    [],
  );

  const performSave = useCallback(async () => {
    if (publishRef.current) return;
    const current = noteRef.current;
    const validation = validate(current);
    if (!validation.ok) {
      setError(validation.error || "Validation failed");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await save(current);
      setDirty(false);
      setLastSavedAt(new Date());
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save";
      setError(message);
    } finally {
      setSaving(false);
    }
  }, [save, validate]);

  // Debounced autosave.
  useEffect(() => {
    if (!dirty || saving || publishRef.current) return undefined;
    const timer = setTimeout(() => {
      void performSave();
    }, delayMs);
    return () => clearTimeout(timer);
  }, [dirty, saving, delayMs, performSave, publishInFlight]);

  // Clear validation error when inputs become valid.
  useEffect(() => {
    const validation = validate(noteRef.current);
    if (validation.ok && error) {
      setError(null);
    }
  }, [validate, note, error]);

  const triggerSaveNow = useCallback(() => {
    if (saving) return;
    void performSave();
  }, [performSave, saving]);

  return {
    note,
    setNote: updateNote,
    dirty,
    saving,
    error,
    lastSavedAt,
    triggerSaveNow,
  };
}
