---
id: REQ-JS-015
title: SvelteKit $lib alias
scope: js
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve `$lib` and `$lib/…` to `src/lib` in the
directory of the nearest `svelte.config.js` (`.mjs`, `.cjs`, `.ts`) above
the importing file, like a relative specifier (REQ-JS-002).

## Rationale

SvelteKit declares the alias in a `tsconfig.json` it generates under
`.svelte-kit/`, which is not committed, and every SvelteKit project imports its
own library through it.

## Acceptance criteria

1. In `testdata/sveltekit`, `$lib` resolves to `src/lib/index.ts` and
   `$lib/format` to `src/lib/format.ts`.
2. `$app/navigation` and `$env/dynamic/public` are dropped.

## Notes

A `kit.alias` or `kit.files.lib` set in `svelte.config.js` is not read.
