---
id: REQ-CUE-004
title: Module-local packages
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import whose path starts with the module path of the governing
`cue.mod/module.cue` (major version suffix removed) **shall** resolve to
the files of the directory under the module root that belong to the
package the import names (its `:qualifier`, else its last element), one
import per file (at most 64); the module is the nearest enclosing one
whose module path matches, else any module of the repository with the
longest matching path. A package of the module that is not there
**shall** be dropped.

## Rationale

A CUE package is a directory and a package name; a package split over
files is imported as a whole.

## Acceptance criteria

1. `example.com/shop/schema` links to both files of package `schema`,
   `:other` to the third file, `nothere` is dropped, and the module with an
   empty `module.cue` reaches the root module's package.
