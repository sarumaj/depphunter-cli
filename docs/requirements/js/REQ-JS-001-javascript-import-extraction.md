---
id: REQ-JS-001
title: JavaScript and TypeScript import extraction
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The JavaScript/TypeScript plugin **shall** claim non-binary files with the
extensions `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.mts`, `.cts` and `.tsx`,
except a `.ts` file the scan found to be XML (a Qt Linguist translation,
REQ-LANG-015), parse each with the matching grammar (JavaScript, TypeScript
or TSX), and extract as imports the module specifiers of `import`
declarations, `export … from` declarations, `require("…")` calls, dynamic
`import("…")` calls and TypeScript `import x = require("…")`. Vue, Svelte and
Astro components are read the same way, block by block (REQ-JS-012).

## Rationale

CommonJS and dynamic imports are as common in real projects as ES module
declarations. A Qt project's translations are `.ts` files too; parsed as
TypeScript, each one ran into the parse time bound (REQ-LANG-011) and yielded
nothing, which made the analysis of flameshot's 49 translations take 39 s.

## Acceptance criteria

1. `require('fs')` and `import('./lazy.mjs')` are extracted as imports.
2. A `.tsx` file is parsed with the TSX grammar.
3. A Qt Linguist `.ts` translation is not claimed.
