---
id: REQ-NIM-007
title: Packages by their modules
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import found in no directory of the repository and not in the standard
library (and every `pkg/x`) **shall** go to the nimble package whose
installed files have the module: what nimble installed for the project
(`nimbledeps/pkgs2/<name>-<version>-<checksum>/` and `pkgs/`, keeping
`srcDir`), the checkouts Atlas cloned into the project's deps directory
(`atlas.config`'s `deps`, the checkout's repository from its
`.git/config`), the packages `nimble.paths` names, and the packages of the
nimble directory (`NIMBLE_DIR`, else `~/.nimble`: `pkgs2/` and `pkgs/`,
only those a manifest names, the locked version else the newest). Else the
requirement or lock entry the module's first segment names (compared
without case, `_` and `-`, a repository's `nim-` prefix or `-nim` suffix:
`nim-widgets` is `widgets`) **shall** be the package; a missing module of
the package itself or of a project directory is dropped; anything else is
an unresolved nimble package named by its first segment. An installed
package's `.nimble` requirements **shall** be its dependencies when no lock
names it.

## Rationale

A module's name need not be its package's (sdl2_nim ships `sdl2`); what is
installed says which package has it.

## Acceptance criteria

1. The fixture's `sdl2` is sdl2_nim from the nimble directory, `httputils`
   the installed package no requirement names, `malebolgia/lockers` Atlas's
   checkout, and `missing/thing` an unresolved package `missing`.
