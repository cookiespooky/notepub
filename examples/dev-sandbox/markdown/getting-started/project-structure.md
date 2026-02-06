---
title: "Структура проекта на пальцах"
type: guide
slug: "project-structure"
description: "Где что лежит и как это связано."
hub: "[[getting-started]]"
---

У Notepub понятная география:

- Контент хранится в Markdown‑файлах.
- `rules.yaml` описывает типы и маршруты.
- Темы лежат в `themes/` или в указанной директории и содержат HTML‑шаблоны.
- `artifacts/` — служебные файлы индекса (sitemap, search, resolve).
- `dist/` — результат `notepub build`.

Главное помнить: правила и контент — источник истины, а артефакты и сборка — производные.
