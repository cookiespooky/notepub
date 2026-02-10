---
type: page
slug: jsonld
title: "JSON-LD Example"
description: "Shows JSON-LD injection from frontmatter."
jsonld: |
  {
    "@context": "https://schema.org",
    "@type": "Article",
    "headline": "JSON-LD Example"
  }
---
JSON-LD is injected into the head when valid JSON is provided.
