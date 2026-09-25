---
id: REQ-FND-006
uuid: 021ac93e-87c5-4898-a40a-77e30a7859fa
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
