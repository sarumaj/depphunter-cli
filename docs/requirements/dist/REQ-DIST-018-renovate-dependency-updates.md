---
id: REQ-DIST-018
uuid: bb33f3dc-052b-4890-b6c1-3d8e1e452d49
title: Automated dependency updates
scope: dist
type: non-functional
priority: should
status: implemented
source:
  - docs/REQUIREMENTS.md M9
verification:
  - inspection
---

## Statement

Dependency updates **should** be proposed as pull requests by Renovate,
configured in `renovate.json` with the recommended preset and all non-major
updates grouped; an update that needs a newer Go than `go.mod` states is
rejected by the oldest-Go CI job (REQ-DIST-011).

## Rationale

Automated, grouped updates keep dependencies current with few pull requests.

## Acceptance criteria

1. `renovate.json` extends `config:recommended` and `group:allNonMajor`.
2. Vendored web libraries and test data are ignored.

## Notes

`renovate.json` is JSON and cannot carry an annotation, so this requirement has
no `Implements:` location.
