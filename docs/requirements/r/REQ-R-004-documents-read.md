---
id: REQ-R-004
title: R Markdown and Quarto chunks read
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read the R chunks of `.Rmd` and `.qmd` documents
(```` ```{r ...} ```` to the closing fence), each chunk lexed on its own with
the document's line numbers, skip chunks of other engines and fenced blocks
that only show code, and import the documents a chunk's `child` option or a
Quarto `{{< include >}}` shortcode pulls in.

## Rationale

Vignettes and reports hold as much of a project's R code as its scripts.

## Acceptance criteria

1. In `vignettes/intro.Rmd`, `library()` calls of R chunks are imports, the
   python chunk and a chunk shown inside a markdown fence are not, and
   `plot_total` is on its document line.
2. `child = "details.Rmd"` and `{{< include _setup.qmd >}}` resolve to the
   files.
