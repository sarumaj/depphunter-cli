---
id: REQ-LANG-010
title: Parallel bounded parsing
scope: lang
type: non-functional
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

The system **shall** read and parse the claimed files of a plugin in parallel,
with at most as many concurrent workers as the machine has logical CPUs.

## Rationale

The pure-Go runtime is about 20 times slower than the C one, so analysis time
depends on using every core; a bound keeps memory use predictable.

## Acceptance criteria

1. Files are parsed concurrently by a worker pool limited to `runtime.NumCPU()`
   workers.
2. A canceled analysis stops scheduling further files.
