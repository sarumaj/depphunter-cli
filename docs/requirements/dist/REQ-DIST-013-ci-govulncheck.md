---
id: REQ-DIST-013
uuid: 1144993c-7c61-4057-903a-e0ff50261bb8
title: CI vulnerability check
scope: dist
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - inspection
---

## Statement

CI **shall** run govulncheck against the code built with the current stable Go
and fail on a reachable known vulnerability.

## Rationale

Release binaries are built with the current Go, so that is the build users run
and the one to scan.

## Acceptance criteria

1. The `vulncheck` job runs `govulncheck ./...` with Go `stable`.
