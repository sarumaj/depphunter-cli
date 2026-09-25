---
id: REQ-DIST-014
uuid: 64078407-3c22-45a7-85a5-5c8c7c992508
title: CI builds every release target
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

CI **shall** build the archives of every release target with `scripts/dist.sh`
on each push and pull request, smoke-test the Linux amd64 binary and keep the
archives as workflow artifacts.

## Rationale

A target that stops cross-compiling fails at once rather than at tag time, and
unreleased builds can be tried.

## Acceptance criteria

1. The `dist` job runs `scripts/dist.sh` and uploads `dist/` as an artifact kept
   for 14 days.

## Notes

CI runs on pushes to `main` and on pull requests, not on pushes to other
branches.
