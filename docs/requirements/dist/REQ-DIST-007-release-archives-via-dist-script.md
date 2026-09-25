---
id: REQ-DIST-007
uuid: b3686f3a-bcf2-4aed-ae8f-453eedd80997
title: Release archives built by scripts/dist.sh
scope: dist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §6
  - docs/REQUIREMENTS.md M7
verification:
  - integration
  - inspection
---

## Statement

`scripts/dist.sh [version]` **shall** cross-compile an archive for each of Linux
(amd64, arm64, armv7, 386, riscv64), macOS (amd64, arm64), Windows (amd64,
arm64, 386) and FreeBSD (amd64, arm64) into `dist/`, each holding the binary,
`README.md`, `LICENSE` and the vendored libraries' licenses, zip for Windows and
gzipped tar otherwise, together with a `checksums.txt` of SHA-256 sums.

## Rationale

One script builds every release target from any host, so CI and a developer
produce the same archives.

## Acceptance criteria

1. `scripts/dist.sh v1.2.3` produces the 12 archives named
   `depphunter_1.2.3_<os>_<arch>` and `checksums.txt`.
2. `TARGETS="linux/amd64"` restricts the build to that target.
