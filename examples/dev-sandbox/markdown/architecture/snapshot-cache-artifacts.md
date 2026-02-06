---
title: "Snapshot, cache, artifacts"
type: guide
slug: "snapshot-cache-artifacts"
description: "Артефакты сборки и их назначение."
hub: "[[architecture]]"
---

В Notepub есть три уровня устойчивости:

- **Snapshot** — снимок объектов (`snapshot/objects.json`), который используется для диффа и повторяемости индексации.
- **Artifacts** — результат индексации: `resolve.json`, `search.json`, sitemap, robots и (опционально) коллекции.
- **Cache** — ускорение повторных операций, чтобы не пересчитывать лишнее.

Эти слои делают систему быстрой и предсказуемой в больших проектах.
