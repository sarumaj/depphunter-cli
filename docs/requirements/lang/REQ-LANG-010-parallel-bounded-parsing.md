---
id: REQ-LANG-010
uuid: 2c289fde-8db5-4f42-9f30-e49e04ddd861
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
2. A cancelled analysis stops scheduling further files.
