---
id: REQ-LSP-009
uuid: 4b997ff5-abeb-4141-877f-b1336918c8e6
title: Time budget for language servers
scope: lsp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
  - docs/REQUIREMENTS.md M9
verification:
  - inspection
---

## Statement

The system **shall** bound the reference search by the `--lsp-timeout` budget
(default 5 minutes) and, when it expires, keep the references found so far and
mark the result partial.

## Rationale

A slow server must not hold the dataset back indefinitely.

## Acceptance criteria

1. With `--lsp-timeout 1s` on a large project, `/api/references` reports
   `partial: true` and the status bar shows "(partial)".
