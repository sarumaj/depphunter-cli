---
id: REQ-MOD-009
uuid: 370b5858-6428-4e24-9fdc-46b14aa175da
title: Transitive package flag
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package node **shall** be able to carry the boolean `transitive`, marking a
package that no file of the project imports.

## Rationale

A package added by walking the dependencies of dependencies has to be
distinguishable from one the project uses itself.

## Acceptance criteria

1. A package added by `--resolve-depth` carries `transitive: true`.
2. A package imported by a project file does not carry `transitive`.

## Notes

When packages are added and marked is specified in scope `sup`.
