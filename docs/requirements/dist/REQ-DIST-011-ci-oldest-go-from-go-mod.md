---
id: REQ-DIST-011
uuid: cf658557-55a7-42bb-98fa-11ea027abba7
title: CI on the oldest supported Go
scope: dist
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
  - docs/REQUIREMENTS.md M9
verification:
  - inspection
---

## Statement

CI **shall** build and test the code with exactly the Go version stated in
`go.mod`, with `GOTOOLCHAIN=local`, so that a change or a dependency update that
needs a newer Go fails.

## Rationale

`go.mod` states the oldest supported Go; without the job it could claim less
than the code and its dependencies need, and `GOTOOLCHAIN=local` stops Go from
silently downloading a newer toolchain.

## Acceptance criteria

1. The `test` job has an `oldest` entry that reads the Go version from `go.mod`.
2. `GOTOOLCHAIN` is `local` for the job.

## Notes

M6 pinned CI to Go 1.22. The dependency refresh after M9 (viper 1.21,
`golang.org/x/*` of 2026) needs Go 1.26 or newer, so Go 1.22 support ended;
`go.mod` states Go 1.27.1 and the job reads the version from it.
