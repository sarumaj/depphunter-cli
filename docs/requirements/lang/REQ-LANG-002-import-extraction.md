---
id: REQ-LANG-002
uuid: cafcde6b-3430-4443-a3a1-0b37572dcfd5
title: Import extraction
scope: lang
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

A plugin **shall** extract from each claimed file its imports, each with the
specifier as written and the 1-based line it appears on.

## Rationale

The raw specifier and its line are what resolution works from and what the side
panel and the open-in-editor action show.

## Acceptance criteria

1. The Go plugin reports the import path and line of every import of a file.
2. The line reported for an import is the line of the import in the source file.
