---
id: REQ-FND-023
title: Findings refreshed in watch mode
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
  - e2e
---

## Statement

In `--watch` mode the system **shall** watch the named reports and **shall**
read the findings again after every re-analysis, whether or not the graph
changed.

## Rationale

A scanner rewriting its report is a finding fixed or a new one; a linter
complaint can go away without the imports changing.

## Acceptance criteria

1. Rewriting a named report with a finding removed updates the map without a
   restart.
2. Only the named report files count as changes in their directory, unless the
   pattern is a glob.
