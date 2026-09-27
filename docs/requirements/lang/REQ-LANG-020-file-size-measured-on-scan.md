---
id: REQ-LANG-020
title: File size measured on scan
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** measure every scanned file's size in bytes and **shall**
carry it into the file's node of the graph.

## Rationale

Size in bytes is available for every file, including files whose lines are never
counted.

## Acceptance criteria

1. Every file node's `bytes` equals the file's size on disk at scan time.
