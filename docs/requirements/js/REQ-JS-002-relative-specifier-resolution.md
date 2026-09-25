---
id: REQ-JS-002
uuid: 5039f77b-a745-4886-b116-d3bf6a1acf09
title: Relative specifier resolution
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a relative specifier to a project file by trying
the path itself, then the path with the extensions `.ts`, `.tsx`, `.d.ts`,
`.js`, `.jsx`, `.mjs`, `.cjs`, `.mts`, `.cts`, `.json` (also after removing a
`.js`/`.jsx`/`.mjs`/`.cjs` extension), then `index` with those extensions inside
the directory, then the directory itself; a relative specifier that leaves the
project or matches nothing **shall** be dropped.

## Rationale

TypeScript sources import `./util.js` for `util.ts` under ESM rules, and
directory imports resolve to their `index` file.

## Acceptance criteria

1. `./util.js` and `./util` resolve to `src/util.ts`.
2. `./components` resolves to `src/components/index.tsx`.
3. `../outside` from the project root yields no target.
