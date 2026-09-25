---
id: REQ-SUP-008
uuid: 4e490854-c4a9-48fe-bfb7-ba2f5ab00c9b
title: Transitive resolution level by level
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

When `--resolve-depth` is N greater than 0, the walker **shall** add the
dependencies of external packages level by level for N levels past the direct
dependencies; with -1 it **shall** continue until a level adds nothing; with 0
it **shall** add nothing.

## Rationale

The map shows what the code imports; the supply chain continues past it, and how
far to follow it is the user's choice.

## Acceptance criteria

1. `--resolve-depth 1` adds exactly the packages the lock file names as the
   direct dependencies' own.
2. `--resolve-depth 2` adds one further level, and `-1` adds every level the
   answers reach.
3. Each package is asked about at most once per analysis, so a cycle in a lock
   file ends the walk.
4. A value below -1 is rejected.
