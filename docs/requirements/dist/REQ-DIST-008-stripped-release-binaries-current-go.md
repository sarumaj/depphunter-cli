---
id: REQ-DIST-008
uuid: ada6a56a-a64e-4988-9824-23c39adef67e
title: Stripped release binaries built with the current Go
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

Release binaries **shall** be built with the current stable Go release, stripped
of symbol and debug tables (`-s -w`), with `-trimpath`, and with the release tag
as their version.

## Rationale

govulncheck found 35 reachable standard-library issues in a build with Go 1.22.2
and none with the current release; the oldest supported Go only states what can
build the code.

## Acceptance criteria

1. The release workflow sets up Go `stable`.
2. `scripts/dist.sh` builds with `-trimpath -ldflags "-s -w -X
   main.version=<version>"`.
