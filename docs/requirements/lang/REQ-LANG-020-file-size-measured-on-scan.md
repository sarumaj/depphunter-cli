---
id: REQ-LANG-020
uuid: f365f3c0-4233-491d-9fbe-9541a3994ee1
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
