---
id: REQ-FND-021
uuid: 2ca55f25-af50-47c2-8ce1-9bca366d8cfe
title: Directory badge with the worst finding below
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

A directory, file, ecosystem or package **shall** carry, as a badge beside its
name in the side panel, the worst severity and the count of the findings on it
and everything below it.

## Rationale

A collapsed directory must say that something below it needs attention.

## Acceptance criteria

1. Selecting a directory with a high finding two levels down shows a `high · 1`
   badge.
