---
id: REQ-TRC-013
uuid: 245306b5-f862-42d7-86b5-33abb1d7f458
title: Resolution report as a text digest
scope: trc
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
  - integration
---

## Statement

`GET /api/resolution?format=text` **shall** serve the same text digest that
`--explain` writes, rendered by the same Go code.

## Rationale

The report is rendered once, in Go, and read three ways.

## Acceptance criteria

1. The text response and the `--explain` output of the same analysis are
   identical apart from the log framing.
2. The document the editor opens and the digest `--explain` writes describe the
   same analysis.
