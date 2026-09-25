---
id: REQ-FND-020
uuid: 256ca82e-4770-4d87-b623-36da599f6e2e
title: Findings placed where they belong
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

The system **shall** place a vulnerability on the package it affects, a linter
finding on the file it is about, and a finding that names both on both; a
finding with a path not on the map **shall** go to the nearest directory that
is, and a finding with neither to the repository root.

## Rationale

The map shows a finding where the reader would look for it.

## Acceptance criteria

1. A govulncheck finding with a call site appears on both the Go package and the
   calling file.
2. An osv-scanner finding appears on its package.
