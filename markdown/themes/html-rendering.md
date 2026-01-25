---
title: "Как работает HTML-рендер"
type: guide
slug: "html-rendering"
description: "От Markdown и правил к готовой странице."
hub: "[[themes]]"
---

Рендер в Notepub — это последовательный процесс:

1) Индексация: чтение Markdown, разбор frontmatter и построение `resolve.json`, `search.json`, `sitemap`.
2) Выбор типа: `type` из frontmatter определяет шаблон и permalink.
3) Рендер: Markdown превращается в HTML‑`Body`, затем вставляется в шаблон темы.

Один и тот же контент может быть отрендерен сервером (`serve`) или статически (`build`). Итог всегда HTML, независимый от среды.
