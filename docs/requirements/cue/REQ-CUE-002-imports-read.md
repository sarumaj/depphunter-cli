---
id: REQ-CUE-002
title: Imports read
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every import declaration (`import "path"`, `import alias "path"` and
grouped `import ( ... )`) **shall** be read, the path as written (a
`:package` qualifier included) as its spec, and nothing inside a comment
or a string (quoted, multi-line `"""`/`'''`, `#`-delimited raw strings,
interpolations) nor a field named `import` **shall** be.

## Rationale

Imports are declarations at the top of a file; text that looks like one
elsewhere is data.

## Acceptance criteria

1. The fixture's imports are read, and those written in comments, strings,
   raw and multi-line strings and interpolations are not.
