---
id: REQ-PERF-015
title: A change of depth stays responsive
scope: perf
type: non-functional
priority: should
status: implemented
verification:
  - e2e
  - manual
---

## Statement

Stepping the depth (REQ-MAP-023) **shall** show the new depth on the toolbar
before the map is laid out, and **shall** lay the map out once for the depth
reached by the steps taken meanwhile, not once per step. Out of walk mode, the
street's trees, lamps and ramps **shall** be built after the new layout has
been drawn once, a few milliseconds a frame, and **shall** not be finished for
a layout replaced before then; in walk mode, and on entering it, they **shall**
be built at once, and **shall** be the same props whichever way they were
built. The ground's
geometry **shall** be written into typed arrays sized beforehand.

## Rationale

On a large repository a layout takes long enough that pressing + three times
froze the page three times over, for depths nobody meant to stop at, and the
street furniture cost more than the rest of the layout together.

## Acceptance criteria

1. On a repository of 16,800 files, a press of + or - returns in a few
   milliseconds, and three presses in a row give one layout.
2. The main thread's work before the new layout is drawn is about a quarter of
   what it was (0.85 s to 0.23 s a layout on that repository).
3. The ground geometry is identical, attribute by attribute, to what it was
   before the change.
4. The finished map is drawn pixel for pixel as before.
5. On a repository of 4,000 boxes at the depth shown, no frame spends more
   than about 15 ms building props, where building them took 0.35 s in one go.

## Notes

The map's cursor shows progress while a layout is due.
