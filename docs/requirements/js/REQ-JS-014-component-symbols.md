---
id: REQ-JS-014
uuid: 0387387f-ff6f-46bc-a498-e64221b9febc
title: Components are symbols
scope: js
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** report a Vue, Svelte or Astro component as a symbol of kind
`component` named after its file without the extension, on line 1, followed by
the symbols of REQ-JS-011 from every block REQ-JS-012 reads.

## Rationale

A component is what other files import it as, under its file name, and its
functions and constants are what a reader navigates by, as in any script.

## Acceptance criteria

1. `src/ui/Counter.vue` yields `Counter` (component), `count` and `closing`
   (var) and `increment` (func, line 24).
2. `src/ui/Widget.svelte` yields `preload` from its module script and
   `name` and `greet` from its instance script.
