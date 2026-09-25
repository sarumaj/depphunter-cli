---
id: REQ-TOOL-050
uuid: db0dcb68-9545-460f-9dd4-f77be9ed3d8d
title: An empty tool stays stopped until re-equipped
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M25
verification:
  - e2e
---

## Statement

A tool whose tank has run dry in use **shall** stay stopped until it is put away
and taken out again, even as its tank refills.

## Rationale

An empty jet that keeps catching is worse than one that has plainly stopped.

## Acceptance criteria

1. A tank that has run dry needs re-equipping once it is empty (M25 acceptance).
