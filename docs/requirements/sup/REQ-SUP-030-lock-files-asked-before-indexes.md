---
id: REQ-SUP-030
uuid: a91ae44a-65ca-4fd6-87bd-6420116b574a
title: Lock files are asked first
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The walker **shall** ask the repository's lock files first and **shall** ask an
index only when they do not answer for a package.

## Rationale

A lock file is both faster and more truthful about this project than a registry.

## Acceptance criteria

1. A package the lock file answers for is recorded as answered by the lock file
   and produces no index request, with or without `--online`.
