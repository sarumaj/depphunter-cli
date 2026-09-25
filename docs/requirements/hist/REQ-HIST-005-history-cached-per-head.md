---
id: REQ-HIST-005
uuid: d2a08a62-32cb-4a1b-948d-29300f194c4e
title: History cached per HEAD
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - integration
---

## Statement

The system **shall** cache the history per project, HEAD commit and commit limit
next to the analysis cache, reuse it while HEAD is unchanged, and remove the
cached histories of older HEADs of the same project.

## Rationale

A repeated run on an unchanged repository then costs no git call beyond
`rev-parse`.

## Acceptance criteria

1. A second read at the same HEAD returns the cached history without running
   `git log`.
2. After a new commit the history is read again and the older cache file is
   removed.
