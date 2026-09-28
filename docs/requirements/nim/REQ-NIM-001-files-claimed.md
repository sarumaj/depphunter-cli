---
id: REQ-NIM-001
title: Files claimed
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Nim plugin **shall** claim Nim modules (`.nim`), NimScript (`.nims`:
`config.nims` and task scripts), nimble's package files (`.nimble`) and
lock file (`nimble.lock`), Atlas's lock file (`atlas.lock`) and the
compiler's configurations (`nim.cfg`, `<name>.nim.cfg`), the manifests
told apart from other files by their names (a `.nimble` file's class
carrying its name, the package's name). Nothing in a `nimbledeps`
directory (what nimble installs for a project), a `nimcache` directory (the
compiler's generated C) or a `deps` directory beside a `.nimble` file, an
`atlas.config` or an `atlas.workspace`, or holding an `atlas.config` (the
packages Atlas cloned), **shall** be claimed, and a walk of the file system
**shall** not enter them.

## Rationale

nimble and Atlas install dependencies into the project; their sources are
the packages', not the project's, and are read only to index their modules
(REQ-NIM-007).

## Acceptance criteria

1. The fixture's modules, NimScript files and manifests are claimed; its
   `nimbledeps/pkgs2/...`, `atlasapp/deps/...` and `nimcache/...` files
   are not, and `setup.cfg` is not.
