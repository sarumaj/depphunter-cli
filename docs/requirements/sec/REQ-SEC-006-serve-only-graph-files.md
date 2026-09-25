---
id: REQ-SEC-006
uuid: afb75983-384a-4c79-bc26-1eb08b89b6cf
title: Only graph files served
scope: sec
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §6
verification:
  - integration
---

## Statement

The server **shall** serve through `/api/file` only files that are part of the
analyzed graph, identified by their exact graph path, and **shall** answer 404
for any other path.

## Rationale

An allow-list rules out path traversal and keeps files the analysis excluded
(secrets, ignored files) out of the browser.

## Acceptance criteria

1. `/api/file?path=a.go` for a graph file answers 200 with its content.
2. `secret.txt` (not in the graph), `../a.go`, `/etc/passwd` and `./a.go` answer
   404.
