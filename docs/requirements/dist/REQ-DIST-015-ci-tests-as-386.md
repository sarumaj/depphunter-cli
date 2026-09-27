---
id: REQ-DIST-015
title: CI tests on 32-bit
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

CI **shall** run the Go tests compiled for `GOARCH=386`.

## Rationale

Releases include 32-bit builds (386, armv7); the job catches integer overflows
on 32-bit `int`.

## Acceptance criteria

1. The `dist` job runs `GOARCH=386 CGO_ENABLED=0 go test ./...`.
