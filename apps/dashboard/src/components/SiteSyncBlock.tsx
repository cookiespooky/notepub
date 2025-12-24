"use client";

import { useEffect, useState, useTransition } from "react";
import styles from "@/app/dashboard/sites/sites.module.css";

type Token = {
  id: string;
  label: string | null;
  createdAt: string;
  lastUsedAt: string | null;
  isActive: boolean;
};

export function SiteSyncBlock({ siteId, siteSlug }: { siteId: string; siteSlug: string }) {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [newLabel, setNewLabel] = useState("");
  const [rawToken, setRawToken] = useState("");
  const [pending, startTransition] = useTransition();
  const [showRevoke, setShowRevoke] = useState(false);
  const [justCreatedToken, setJustCreatedToken] = useState("");
  const [copied, setCopied] = useState(false);

  const loadTokens = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`/api/sync-tokens?siteId=${siteId}`, { cache: "no-store" });
      if (!res.ok) throw new Error("Не удалось загрузить токены");
      const data = await res.json();
      setTokens(data.tokens || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка загрузки");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTokens();
  }, [siteId]);

  const createToken = async () => {
    setError("");
    setRawToken("");
    startTransition(() => {
      void (async () => {
        try {
          const res = await fetch("/api/sync-tokens", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ label: newLabel.trim() || undefined, siteId }),
          });
          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error(data.error || "Не удалось создать токен");
          }
          const data = await res.json();
          setRawToken(data.token);
          setJustCreatedToken(data.token);
          setCopied(false);
          setNewLabel("");
          await loadTokens();
        } catch (err) {
          setError(err instanceof Error ? err.message : "Ошибка создания токена");
        }
      })();
    });
  };

  const toggleToken = async (id: string, isActive: boolean) => {
    setError("");
    startTransition(() => {
      void (async () => {
        try {
          const res = await fetch(`/api/sync-tokens/${id}`, {
            method: "PATCH",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ isActive: !isActive, siteId }),
          });
          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error(data.error || "Не удалось обновить токен");
          }
          await loadTokens();
        } catch (err) {
          setError(err instanceof Error ? err.message : "Ошибка обновления токена");
        }
      })();
    });
  };

  const revokeToken = async (tokenId: string) => {
    await toggleToken(tokenId, true);
    setShowRevoke(false);
  };

  const activeToken = tokens.find((t) => t.isActive);
  const showCreate = !activeToken;

  return (
    <>
      <div className={styles.card}>
        <h3>Подключение Obsidian (Remotely Save)</h3>
        <p className={styles.meta} style={{ marginBottom: 20 }}>▶️ Настройте синхронизацию по <a href="https://about.notepub.site/manuals/obsidian-sync" target="_blank" rel="noopener noreferrer">этой инструкции</a>.</p>
        <div className={styles.cardFlex}>
          {showCreate && (
            <div className={styles.form}>
              <label className={styles.field}>
                <span>⚠️ Токен показывается только 1 раз при создании. Сохраните его в надежном месте и не передавайте никому.</span>
                <p style={{ marginBottom: 0 }}>Название токена</p>
                <input value={newLabel} onChange={(e) => setNewLabel(e.target.value)} placeholder="Любое для вашего удобства" />
              </label>
              <button type="button" className={styles.primary} onClick={createToken} disabled={pending}>
                {pending ? "Создаем..." : "Создать токен"}
              </button>
              {error && <p className={styles.error}>{error}</p>}
            </div>
          )}

          <div style={{ flex: 1 }}>
            <div className={styles.formDescription}>
            </div>
            <div className={styles.list}>
              {justCreatedToken && (
                <div className={styles.tokenCard}>
                  <div className={styles.tokenGenerated}>Токен (показан один раз): <code>{justCreatedToken}</code></div>
                  <div style={{ display: "flex", gap: 10, marginTop: 6, alignItems: "center" }}>
                    <button
                      type="button"
                      className={styles.secondary}
                      onClick={() => {
                        navigator.clipboard.writeText(justCreatedToken).then(() => {
                          setCopied(true);
                          setTimeout(() => setCopied(false), 2000);
                        });
                      }}
                    >
                      Скопировать
                    </button>
                    {copied && <span className={styles.meta}>Скопировано</span>}
                  </div>
                </div>
              )}
              {loading ? (
                <p>Загрузка...</p>
              ) : tokens.length === 0 ? (
                <p className={styles.meta}>Токенов пока нет.</p>
              ) : (
                tokens.map((token) => (
                  <div key={token.id} className={styles.tokenCard}>
                    <div className={styles.cardHeader}>
                      <strong>{token.label || "Без метки"}</strong>
                      <span className={styles.pill}>{token.isActive ? "Активен" : "Отключен"}</span>
                    </div>
                    <div className={styles.meta}>
                      Создан: {formatDate(token.createdAt)} {token.lastUsedAt ? `• Последнее использование: ${formatDate(token.lastUsedAt)}` : ""}
                    </div>
                    {token.isActive && (
                      <div style={{ display: "flex", gap: 10 }}>
                        <button
                          type="button"
                          className={styles.secondary}
                          onClick={() => setShowRevoke(true)}
                          disabled={pending}
                        >
                          Отключить
                        </button>
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>

      {showRevoke && activeToken && (
        <div className={styles.modalOverlay} role="dialog" aria-modal="true">
          <div className={styles.modal}>
            <h3>Отключить токен?</h3>
            <p>Синхронизация с текущим токеном будет остановлена. После этого вы сможете создать новый токен.</p>
            {error && <p className={styles.error}>{error}</p>}
            <div className={styles.modalActions}>
              <button type="button" className={styles.modalSecondary} onClick={() => setShowRevoke(false)}>
                Отмена
              </button>
              <button
                type="button"
                className={styles.modalPrimary}
                onClick={() => revokeToken(activeToken.id)}
                disabled={pending}
              >
                Отключить
              </button>
            </div>
          </div>
        </div>
      )}

    </>
  );
}

function formatDate(input: string | null) {
  if (!input) return "—";
  const d = new Date(input);
  if (Number.isNaN(d.getTime())) return input;
  return d.toLocaleString();
}
