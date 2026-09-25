---
id: REQ-LANG-021
uuid: d3d35ce1-5522-4eb1-8f0c-bb94c816a8fa
title: Binary files not read
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** classify a file as binary when its first 8 000 bytes
contain a NUL byte, **shall not** count its lines, and **shall not** pass it to
any plugin; it **shall** still be listed.

## Rationale

Binary content has no lines and no imports.

## Acceptance criteria

1. A file containing `\x00` is listed, has no line count and is not parsed.
