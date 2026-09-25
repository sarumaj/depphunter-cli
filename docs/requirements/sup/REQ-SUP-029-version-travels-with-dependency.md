---
id: REQ-SUP-029
uuid: baa2afce-15f1-4075-947b-6b3287c70557
title: The version travels with a dependency
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

Every dependency an index names **shall** carry the version the index gave for
it, so that the next level can be asked for that version; a Go module without a
version **shall not** be asked about.

## Rationale

A module proxy serves a go.mod for a version, not for a module.

## Acceptance criteria

1. A Go dependency returned by the proxy carries its version and is asked about
   at the next level.
2. A Go module without a version is recorded as unanswered because a proxy needs
   a version.
