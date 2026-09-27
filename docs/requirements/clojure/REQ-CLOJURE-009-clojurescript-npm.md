---
id: REQ-CLOJURE-009
title: ClojureScript npm requires
scope: clojure
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

A ClojureScript string require **shall** resolve to the npm package it names
(`"@mui/material/Button"` is `@mui/material`), versioned and pinned
(`lang.PinnedSemver`) by the `package.json` (`dependencies`,
`devDependencies`) or `deps.cljs` `:npm-deps` of a governing project, else
unresolved; a Node module (`"fs"`, `"node:path"`) **shall** be the hidden
`node` island's and a relative one (`"./x.js"`) the project file. A symbol
require that no namespace answers but a governing project's npm
dependencies name (shadow-cljs's `[react :as r]`) **shall** be that npm
package.

## Rationale

shadow-cljs builds require npm packages directly.

## Acceptance criteria

1. In `web/src/shop/ui.cljs`, `"react"` is npm `react` 18.2.0 pinned,
   `"fs"` is `node` `fs`, `"lodash"` is unresolved and `left-pad` (a
   symbol) is npm `left-pad`.
