---
id: REQ-SEC-008
uuid: c6288e5f-f6c4-4869-b249-89bd0200bbee
title: Open in editor only for graph files
scope: sec
type: non-functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

The server **shall** open in the editor only files that are part of the analyzed
graph, and **shall** answer 404 to `POST /api/open` for any other path.

## Rationale

The endpoint executes a program; restricting it to graph files keeps a request
from naming arbitrary paths.

## Acceptance criteria

1. `POST /api/open` for `secret.txt` (not in the graph) answers 404 and runs
   nothing.
2. `POST /api/open` for `a.go` answers 204 and runs the editor.

## Notes

The open-in-editor behavior itself belongs to scope `srv`.
