---
id: REQ-DIST-004
uuid: 79227a53-95f0-4094-bcd9-525e9b039b4f
title: BSD 3-Clause license
scope: dist
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The system **shall** be licensed under the BSD 3-Clause license, stated in the
`LICENSE` file at the root of the repository and included in every release
archive.

## Rationale

A permissive license allows use and redistribution without obligations beyond
keeping the notice.

## Acceptance criteria

1. `LICENSE` holds the BSD 3-Clause text.
2. Every archive built by `scripts/dist.sh` contains `LICENSE`.

## Notes

`LICENSE` is plain text and carries no annotation; the archive step of
`scripts/dist.sh` is annotated.
