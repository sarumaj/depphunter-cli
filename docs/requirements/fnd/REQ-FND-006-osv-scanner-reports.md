---
id: REQ-FND-006
title: osv-scanner reports
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read osv-scanner JSON reports and place their findings on
the affected packages.

## Rationale

osv-scanner is the OSV project's own lock-file scanner.

## Acceptance criteria

1. An osv-scanner report yields findings with ecosystem, package and version of
   the map.
