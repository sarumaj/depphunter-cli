---
id: REQ-JS-001
uuid: c4a383ff-0dee-4337-b492-dff512433a45
title: JavaScript and TypeScript import extraction
scope: js
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M2
verification:
  - unit
---

## Statement

The JavaScript/TypeScript plugin **shall** claim non-binary files with the
extensions `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.mts`, `.cts` and `.tsx`,
parse each with the matching grammar (JavaScript, TypeScript or TSX), and
extract as imports the module specifiers of `import` declarations, `export …
from` declarations, `require("…")` calls, dynamic `import("…")` calls and
TypeScript `import x = require("…")`.

## Rationale

CommonJS and dynamic imports are as common in real projects as ES module
declarations.

## Acceptance criteria

1. `require('fs')` and `import('./lazy.mjs')` are extracted as imports.
2. A `.tsx` file is parsed with the TSX grammar.
