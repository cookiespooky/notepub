---
title: "permalink, collections, include_in"
type: guide
slug: "permalink-collections-include"
description: "Три механизма, которые управляют маршрутизацией и выдачами."
hub: "[[concepts]]"
---

`permalink` определяет структуру URL для каждого типа. В текущих правилах это `"/{{ slug }}/"` для `page` и `hub`, а для контента — `"/docs/{{ slug }}/"` (guide) и `"/blog/{{ slug }}/"` (essay).

`include_in` включает или исключает типы из sitemap и поиска. Например, `hub` попадает в sitemap, но не в search, а `guide` и `essay` — в оба.

`collections` описывают, как собирать выдачи: хабовые списки, related‑соседей, группировки по типам и теги. Это декларативный слой, который не зависит от темы.
