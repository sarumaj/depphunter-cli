---
id: REQ-DIST-001
uuid: fdf5eb5f-85b4-481a-9bdd-ecfa70329ece
title: Single static binary per platform
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - integration
  - inspection
---

## Statement

The system **shall** be distributed as a single, statically linked, pure-Go
binary (built with `CGO_ENABLED=0`) for Linux, macOS, Windows and FreeBSD.

## Rationale

A single binary installs by copying and runs without a runtime or shared
libraries.

## Acceptance criteria

1. `scripts/dist.sh` builds every target with `CGO_ENABLED=0` from one host.
2. The Linux amd64 archive's binary runs `--version` in CI.

## Notes

The architectures are listed in REQ-DIST-007.
