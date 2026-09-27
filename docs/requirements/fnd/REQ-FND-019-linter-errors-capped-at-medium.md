---
id: REQ-FND-019
title: Linter errors capped at medium
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The severity of a golangci-lint or eslint finding **shall** be at most medium: a
linter "error" **shall** be medium and a "warning" low. Trivy misconfiguration
and secret findings keep the severity Trivy assigns.

## Rationale

A linter's error is not the same news as a critical advisory, and the streets
would otherwise fill with bugs that mean a missing comment.

## Acceptance criteria

1. golangci-lint and eslint findings are never high or critical.
