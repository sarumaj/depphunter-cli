---
id: REQ-DHALL-004
title: Local imports
scope: dhall
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A relative import **shall** link to the repository file it names, relative
to the importing file (quoted path segments unquoted). An absolute path, a
path under the home directory, an environment variable and a relative path
to a file that is not in the repository **shall** be dropped.

## Rationale

Only a relative path names a file of the project; the others depend on the
machine the interpreter runs on.

## Acceptance criteria

1. `../lib/util.dhall`, `./schema.dhall` (also `as Location`) and
   `../README.md as Text` link to their files; `env:DHALL_LOCAL`,
   `./missing.dhall`, `/etc/dhall/x.dhall` and `~/x.dhall` are dropped.
