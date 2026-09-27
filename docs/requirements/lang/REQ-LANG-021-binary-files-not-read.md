---
id: REQ-LANG-021
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
