---
id: REQ-TRC-001
uuid: 2378bd67-899c-4e29-8963-36d35189f80d
title: One resolution report per analysis
scope: trc
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
  - integration
---

## Statement

Every analysis **shall** produce one resolution report recording how the
dependencies on the map were reached, including the settings it was run with:
the resolve depth, whether it was online, the private patterns and the
vouched-for indexes.

## Rationale

Dependency resolution leaves no mark on the map: a package from a company Nexus
is drawn like one from registry.npmjs.org, and a tree that stops two levels down
looks the same whether the dependencies end there or a proxy answered 404.

## Acceptance criteria

1. An analysis hands back the report it filled.
2. The report states `--resolve-depth`, `--online`, the private patterns and the
   trusted indexes of the run.
