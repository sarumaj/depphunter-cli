---
id: REQ-LANG-027
uuid: 94ad5073-e630-4e78-a425-e25eba746277
title: Cached runs resolve current manifests
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

On every run the system **shall** resolve imports, including those of cached
extractions, against the manifests and layout of the current run.

## Rationale

A manifest or lock file change alters where an unchanged file's imports point;
caching resolved results would show stale versions.

## Acceptance criteria

1. Changing a version in `go.mod` without changing a Go source file changes the
   version shown for the imported module on the next run.
