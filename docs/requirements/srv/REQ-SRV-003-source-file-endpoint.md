---
id: REQ-SRV-003
uuid: d47194d9-e753-44e4-88a4-e3c5f90c7999
title: Source file endpoint
scope: srv
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M1
verification:
  - integration
---

## Statement

The server **shall** answer `GET /api/file?path=<path>` with the text of the
named project file as `text/plain; charset=utf-8`, for display in the side
panel.

## Rationale

The side panel shows syntax-highlighted source without the browser having
file-system access.

## Acceptance criteria

1. A request for a file node of the graph returns 200 and the file content.
2. A file larger than 4 MiB is answered 413 rather than served.

## Notes

That only files that are part of the graph are served is a security requirement
of scope `sec`.
