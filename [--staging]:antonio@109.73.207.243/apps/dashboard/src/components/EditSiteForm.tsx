"use client";

import { useEffect, useState } from "react";
import { useFormState, useFormStatus } from "react-dom";
import { useRouter } from "next/navigation";
import { updateSiteAction, type UpdateSiteState } from "@/app/dashboard/sites/[id]/actions";
import styles from "@/app/dashboard/sites/sites.module.css";
import { buildSiteHost, getSitesBaseDomain } from "@/lib/domains";

const initialState: UpdateSiteState = { error: "" };
const baseDomain = getSitesBaseDomain();

export function EditSiteForm({
  siteId,
  initialSlug,
  initialTitle,
  initialOgImageUrl,
  initialOgDescription,
  initialHideSidebarOnHome,
}: {
  siteId: string;
  initialSlug: string;
  initialTitle: string;
  initialOgImageUrl?: string | null;
  initialOgDescription?: string | null;
  initialHideSidebarOnHome?: boolean | null;
}) {
  const [state, formAction] = useFormState(updateSiteAction, initialState);
  const router = useRouter();
  const [slug, setSlug] = useState(initialSlug);
  const [title, setTitle] = useState(initialTitle);
  const [ogImageUrl, setOgImageUrl] = useState(initialOgImageUrl || "");
  const [ogDescription, setOgDescription] = useState(initialOgDescription || "");
  const [hideSidebarOnHome, setHideSidebarOnHome] = useState(Boolean(initialHideSidebarOnHome));
  const MAX_SLUG = 30;
  const MIN_SLUG = 3;
  const MAX_TITLE = 80;
  const MAX_DESC = 200;

  useEffect(() => {
    if (state?.success && state.redirectTo) {
      const timer = setTimeout(() => router.push(state.redirectTo as string), 600);
      return () => clearTimeout(timer);
    }
  }, [state?.success, state?.redirectTo, router]);

  return (
    <div className={styles.cardFlex}>
      <form action={formAction} className={styles.form}>
        <input type="hidden" name="id" value={siteId} />
        <label className={styles.field}>
          <span>Субдомен сайта</span>
          <input
            name="slug"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            required
            minLength={MIN_SLUG}
            maxLength={MAX_SLUG}
          />
          <div className={styles.fieldMeta}>
            <small>Используется в домене вашего сайта: subdomain.{baseDomain}</small>
            <span className={styles.counter}>
              {slug.length}/{MAX_SLUG}
            </span>
          </div>
        </label>
        <label className={styles.field}>
          <span>Название сайта</span>
          <input
            name="title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            required
            maxLength={MAX_TITLE}
          />
          <div className={styles.fieldMeta}>
            <span />
            <span className={styles.counter}>
              {title.length}/{MAX_TITLE}
            </span>
          </div>
        </label>
        <label className={styles.field}>
          <span>Изображение сайта</span>
          <input
            name="ogImageUrl"
            value={ogImageUrl}
            onChange={(e) => setOgImageUrl(e.target.value)}
            placeholder="https://..."
          />
          <div className={styles.fieldMeta}>
            <small>Если пусто — используется дефолтная картинка</small>
          </div>
        </label>
        <label className={styles.field}>
          <span>Описание сайта</span>
          <textarea
            name="ogDescription"
            value={ogDescription}
            onChange={(e) => setOgDescription(e.target.value)}
            rows={3}
            maxLength={MAX_DESC}
          />
          <div className={styles.fieldMeta}>
            <span />
            <span className={styles.counter}>
              {ogDescription.length}/{MAX_DESC}
            </span>
          </div>
        </label>
        <label className={styles.switchRow}>
          <span>Скрыть боковое меню на главной</span>
          <label className={styles.switch}>
            <input
              type="checkbox"
              name="hideSidebarOnHome"
              checked={hideSidebarOnHome}
              onChange={(e) => setHideSidebarOnHome(e.target.checked)}
            />
            <span className={styles.slider} aria-hidden="true" />
          </label>
        </label>
        <SubmitButton defaultLabel="Сохранить" state={state} />
      </form>

      <div className={styles.previews}>
        <div className={styles.previewLabel}>Превью</div>
        <div className={styles.preview}>
          <div className={styles.previewHeader}>
            <div className={styles.previewUrl}>https://{buildSiteHost(slug || "mysite")}</div>
          </div>
          <div className={styles.previewCard}>
            <div className={styles.previewCardHeader}>
              <div className={styles.previewImage}>
                {ogImageUrl ? <img src={ogImageUrl} alt={title || "Site preview"} /> : <img src='/logo.png' alt={title || "Site preview"} />}
              </div>
              <div className={styles.previewTitle}>{title || "Заголовок сайта"}</div>
            </div>
          </div>
        </div>
        <div className={styles.preview}>
          <div className={styles.previewOg}>
            <div className={styles.previewOgThumb}>
              {ogImageUrl ? <img src={ogImageUrl} alt={title || "Site preview"} /> : <div className={styles.previewPlaceholder}>No image</div>}
            </div>
            <div className={styles.previewOgBody}>
              <div className={styles.previewOgUrl}>https://{buildSiteHost(slug || "mysite")}</div>
              <div className={styles.previewOgTitle}>{title || "Заголовок сайта"}</div>
              <div className={styles.previewOgDesc}>{ogDescription || "Описание сайта появится здесь."}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function SubmitButton({ defaultLabel, state }: { defaultLabel: string; state: UpdateSiteState }) {
  const { pending } = useFormStatus();
  const [label, setLabel] = useState(defaultLabel);

  useEffect(() => {
    if (pending) {
      setLabel("Сохранение");
      return;
    }
    if (state?.error) {
      setLabel("Ошибка");
    } else if (state?.success) {
      setLabel("Сохранено");
    } else {
      setLabel(defaultLabel);
    }
    const timer = setTimeout(() => setLabel(defaultLabel), 2000);
    return () => clearTimeout(timer);
  }, [pending, state?.error, state?.success, defaultLabel]);

  return (
    <div className={styles.actions}>
      <button type="submit" className={styles.primary} disabled={pending}>
        {label}
      </button>
      {state?.error && <p className={styles.error}>{state.error}</p>}
    </div>
  );
}
