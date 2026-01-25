---
title: "Доступные переменные"
type: guide
slug: "available-variables"
description: "Что доступно в шаблонах и как это использовать."
hub: "[[themes]]"
---

Ниже — практичный список переменных, которые доступны в HTML‑шаблонах тем. Эти данные приходят из рендера и нормализованы для использования в шаблоне.

## Базовые переменные

- `{{ .Title }}` — заголовок страницы (то же, что `.Page.Title`)
- `{{ .Canonical }}` — canonical URL, если рассчитан
- `{{ .Body }}` — HTML‑тело страницы (Markdown уже отрендерен)

## Метаданные страницы

- `{{ .Page.Type }}`
- `{{ .Page.Slug }}`
- `{{ .Page.Title }}`
- `{{ .Page.Description }}`
- `{{ .Page.Canonical }}`
- `{{ .Page.NoIndex }}`

## Нормализованные поля

- `{{ .Core.Type }}`
- `{{ .Core.Slug }}`
- `{{ .Core.Title }}`
- `{{ .Core.Description }}`

## Frontmatter (произвольные поля)

`FM` — это карта frontmatter‑полей. Доступ к ним через `index`:

```
{{ index .FM "tags" }}
{{ index .FM "price" }}
```

Если поле не задано, результат будет `nil` — учитывайте это в шаблоне.

## Коллекции

`Collections` — это результаты коллекций из `rules.yaml`. Имена коллекций совпадают с ключами в `collections:`. Например, для текущих правил доступны:

- `home_hubs`
- `hub_items`
- `related_pages`
- `wiki_neighbors`
- `articles_by_tag` (собирает `guide` и `essay`)
- `hub_items_grouped`

### Итерация по Items

```
{{ range .Collections.hub_items.Items }}
  <a href="{{ .Path }}">{{ .Title }}</a>
{{ end }}
```

Поля элемента:

- `.Path`, `.Type`, `.Slug`, `.Title`, `.Description`
- `.Canonical`, `.UpdatedAt`, `.NoIndex`
- `.FM` (frontmatter карты элемента)

### Группировки (Groups)

Если коллекция использует `group_by`, результат приходит как `Groups`:

```
{{ range .Collections.articles_by_tag.Groups }}
  <h3>{{ .Key }}</h3>
  {{ range .Items }}
    <a href="{{ .Path }}">{{ .Title }}</a>
  {{ end }}
{{ end }}
```

## Meta‑хелперы

- `{{ .Meta.Robots }}`
- `{{ .Meta.OpenGraph }}` (массив пар `Key`/`Value`)
- `{{ .Meta.JSONLD }}`

## Флаги

- `{{ .IsHome }}`
- `{{ .IsSearch }}`
- `{{ .HasHomeCSS }}`

## Минимальная структура темы

- `templates/layout.html` — внешний каркас, обязательно содержит `{{ .Body }}`
- `templates/page.html` / `guide.html` / `essay.html` / `hub.html` / `home.html` — только тело страницы
- ассеты доступны по `/assets/*`
