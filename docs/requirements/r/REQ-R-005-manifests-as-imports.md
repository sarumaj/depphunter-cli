---
id: REQ-R-005
uuid: 5b187295-aa9c-4645-b8c7-e6c1768f4043
title: DESCRIPTION and NAMESPACE imports
scope: r
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a `DESCRIPTION`'s `Depends`, `Imports`,
`LinkingTo`, `Suggests` and `Enhances` entries (without `R`) as imports of the
packages they name, each on its line, and a `NAMESPACE`'s `import()`,
`importFrom()`, `importClassesFrom()` and `importMethodsFrom()` directives as
imports, once per directive and package.

## Rationale

A package's dependencies are declared, not imported file by file; a
package used only through `NAMESPACE` would otherwise not be on the map.

## Acceptance criteria

1. `pkgs/shopr/DESCRIPTION` imports every entry of its fields, and
   `localhelper`'s `Imports: shopr` points at shopr's `DESCRIPTION`.
