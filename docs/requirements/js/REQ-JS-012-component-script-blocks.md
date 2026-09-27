---
id: REQ-JS-012
uuid: 08c6da66-104c-4970-bd02-24bf9e412345
title: Vue, Svelte and Astro script blocks
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The JavaScript/TypeScript plugin **shall** claim non-binary `.vue`, `.svelte`
and `.astro` files and extract imports and symbols, as for REQ-JS-001 and
REQ-JS-011, from the component's own code: every top-level `<script>` of a Vue
or Svelte component (`<script setup>`, `<script context="module">` and
`<script module>` included) parsed as JavaScript, TypeScript (`lang="ts"`) or
TSX (`lang="tsx"` or `"jsx"`), and an Astro component's frontmatter between
its opening `---` fences and each `<script>` in its template that has no
attribute but `src`, both parsed as TypeScript. Lines **shall** be those of the
component file. A `<script>` inside an HTML comment, an attribute value, a
template expression, a Vue `<template>` or a Svelte `<svelte:head>` **shall
not** be read, nor shall a script in another language or of a data type
(`type="application/ld+json"`).

## Rationale

Components hold most of the imports of a Vue, Svelte or Astro application; left
unread, those files stood on the map without edges. Their imports are ordinary
module specifiers, so they go through the same resolver as a `.ts` file.

## Acceptance criteria

1. `src/ui/Counter.vue` yields the imports of both its `<script>` and its
   `<script setup>`, and none of the `<script>` text in its template, comment,
   mustache or style.
2. `src/ui/Widget.svelte` yields the imports of its module and instance
   scripts, and none from `<svelte:head>`, an attribute or an expression.
3. `src/pages/index.astro` yields the frontmatter's imports and those of its
   processed template script, but not those of an `is:inline` script.
4. `./Child.vue` imported on line 18 of `Counter.vue` is reported on line 18.

## Notes

The blocks are found by a small tag scanner, not by a grammar (see
`internal/lang/javascript/component.go`). As in HTML, the first `</script`
ends a script. Not read: `@import` in `<style>`, a Vue `<template src>`,
components that frameworks such as Nuxt register without an import, and
expressions in the template (`{#await import('./x')}`).
