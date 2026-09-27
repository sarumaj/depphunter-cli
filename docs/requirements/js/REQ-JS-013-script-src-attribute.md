---
id: REQ-JS-013
uuid: f1c87120-372e-4c83-ba55-e9636f29d2a1
title: A component's script src is an import
scope: js
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract the `src` attribute of a component's `<script>`
tag that REQ-JS-012 reads as an import on the tag's line, resolved like any
other specifier.

## Rationale

A Vue component can keep its code in a separate file (`<script
src="./x.ts">`), and an Astro page can bundle a script file; either is a
dependency of the component.

## Acceptance criteria

1. `src/ui/Child.vue` imports `./child.ts`, resolved to `src/ui/child.ts`.
2. `src/pages/index.astro` imports `../ui/child.ts` from a `<script src>`.
