---
id: REQ-PERF-014
title: A readout of how the map draws
scope: perf
type: functional
priority: may
status: implemented
verification:
  - ui
  - manual
---

## Statement

When the page's address carries `stats`, the system **shall** show in a corner
of the map the frames drawn a second, the CPU time a frame takes, the
resolution it is drawn at, and the draw calls and triangles of the last
frame, and a switch for the depth pre-pass (REQ-PERF-013).

## Rationale

What a change of rendering is worth depends on the GPU; the readout lets it be
seen on the machine in question, and the switch compares the pre-pass with
its absence there.

## Acceptance criteria

1. The readout is shown only when the address asks for it.
2. It reads out the frame rate, CPU time, resolution, draws and triangles.
