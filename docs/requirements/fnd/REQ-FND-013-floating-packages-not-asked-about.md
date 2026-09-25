---
id: REQ-FND-013
uuid: 98f980d4-59db-48a0-a9e3-0c1fccedae9d
title: Floating packages not asked about
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The system **shall not** ask the vulnerability database about a package that is
not pinned to one version.

## Rationale

A floating package resolves to something else on the next install, so any answer
would be about the wrong version.

## Acceptance criteria

1. A package marked `floating` or without a version is not part of the OSV
   query.
