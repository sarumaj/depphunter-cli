---
id: REQ-DIST-017
uuid: 91adf9c6-0044-401e-ad59-ed51dcbf9e99
title: Established web libraries
scope: dist
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - inspection
---

## Statement

The UI **shall** use the vendored `potpack` for terrace packing and `fzf-for-js`
for fuzzy search instead of local code.

## Rationale

Same policy as REQ-DIST-016, applied to the browser modules.

## Acceptance criteria

1. `web/static/layout.js` imports `vendor/potpack.js`.
2. `web/static/filter.js` imports `vendor/fzf.es.js`.
