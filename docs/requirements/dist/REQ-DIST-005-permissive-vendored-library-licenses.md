---
id: REQ-DIST-005
uuid: 5d396b23-38ca-44b4-bee4-984e0ef26ce0
title: Permissive licenses of vendored libraries
scope: dist
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §6
  - docs/REQUIREMENTS.md M7
verification:
  - inspection
---

## Statement

Every vendored and linked library **shall** carry a permissive license
compatible with BSD 3-Clause (such as MIT, BSD 3-Clause, ISC or CC0), and every
vendored web library **shall** be listed with its source, version and license in
`web/static/vendor/README.md`, with its license text beside it.

## Rationale

Compatible licenses keep the product redistributable under its own license.

## Acceptance criteria

1. `web/static/vendor/README.md` lists three.js (MIT), highlight.js (BSD
   3-Clause), potpack (ISC) and fzf (BSD 3-Clause), and the models' sources
   (MIT, CC0).
2. Each listed license file (`*.LICENSE`) exists in `web/static/vendor/`.

## Notes

`web/static/vendor/` is vendored and not annotated.
