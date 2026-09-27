---
id: REQ-ADA-004
title: Units resolved to files
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A with clause **shall** resolve to the file of the repository that
declares the unit (its spec, else a body that is its own declaration),
found by the unit every source declares (every unit of a `.ada` file),
so GNAT's file naming, krunched names and a project's Naming package need
not be followed; the Naming package's `Spec`/`Body` entries **shall**
add the files they name. Of several files declaring a unit, one in a
source directory of a project that governs the importing file (or one it
withs, extends or aggregates) **shall** be preferred, then the nearest. A
body **shall** resolve to its spec, a subunit to its parent's body and a
child to its parent's spec the same way.

## Rationale

GNAT finds units through the project's source directories and naming
scheme; the unit each file declares is the ground truth the scheme only
encodes.

## Acceptance criteria

1. `with Widgets.Buttons` resolves to
   `libs/widgets/src/controls/widgets-buttons.ads`, `with Legacy_IO` to
   `src/legacyio.ads` and `with Tools` to `tools/all_units.ada`.
