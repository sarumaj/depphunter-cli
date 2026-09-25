---
id: REQ-DIST-002
uuid: db81a740-4fd4-4853-a1ba-d534335417fe
title: Frontend embedded in the binary
scope: dist
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §6
verification:
  - inspection
---

## Statement

The binary **shall** embed the browser UI, its vendored libraries and its
models, and serve them from the embedded file system.

## Rationale

The map then needs no files beside the binary and no JavaScript toolchain to
build.

## Acceptance criteria

1. The `web` package embeds `web/static` with `go:embed`.
2. A binary copied alone to another directory serves the map.
