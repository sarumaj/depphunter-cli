---
id: REQ-DIST-010
title: CI tests on Linux, macOS and Windows
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

CI **shall** build and test the code on Linux, macOS and Windows with the
current stable Go, with the race detector wherever the runner supports it.

## Rationale

The tool is cross-platform; path handling and process spawning differ between
systems.

## Acceptance criteria

1. The `test` job runs on `ubuntu-latest`, `macos-latest` and `windows-latest`.
2. Linux and macOS run `go test -race ./...`; Windows runs `go test ./...`.
