---
id: REQ-ADA-006
uuid: e1bb2d01-85cc-4e48-8cfe-5a1b2ec21922
title: Alire manifests and pinning
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every `[[depends-on]]` crate of an `alire.toml` (every alternative of a
`case(...)` expression included), every crate `[[pins]]` pins without
depending on it and every `project-files` entry **shall** be an import.
A crate **shall** be named by its crate name and versioned, for the files
the nearest `alire.toml` above them governs (all manifests for a file
none governs), by: a pin (a directory of the repository is an edge to its
`alire.toml`, a directory outside it floats with the directory as
origin; a git pin pins at its `commit`, floats on its `branch` or
without one; a `version` pin pins), else the lock file's solution
(`alire/alire.lock`, Alire 1.1 and later, else `alire.lock` beside the
manifest) pinning the version it chose or following its link, else the
constraint: `=1.2.3` or a bare `1.2.3` pins and `^`, `~`, `>=`, `/=`,
`*` and `&`/`|` combinations float, with the version Alire fetched shown.
A constraint other than the pinned version **shall** be the requested
one, and a git server other than the public forges the crate's origin.

## Rationale

Alire resolves ranges when it updates the lock file; a pin to a commit is
the only git reference that cannot move.

## Acceptance criteria

1. aunit, gnatcoll and templates_parser are pinned by the lock file with
   their constraints requested, xmlada by `=24.0.0`, ada_toml at its
   commit, acme_log floats on `main` with `https://git.acme.dev/acme_log.git`
   as origin, widgets is `libs/widgets/alire.toml` and win32ada floats.
2. A bare version pins; `^1.2` floats; a lock beside the manifest pins; a
   git pin without a commit floats; a path pin outside the repository
   floats with its path as origin.
