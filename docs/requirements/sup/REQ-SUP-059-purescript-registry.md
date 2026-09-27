---
id: REQ-SUP-059
uuid: e726dd98-fc98-4cf8-83fd-76d2e819b432
title: The PureScript registry for package dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a package of the
`purescript` island depends on from the PureScript registry's repositories,
read as files: the manifest of the version the target names from
`registry-index/main/<shard>/<name>` (one JSON manifest per line, sharded
`1/`, `2/`, `3/<first letter>/` or by the first two and next two letters),
else of the newest version `registry/main/metadata/<name>.json` publishes
that the target's range admits (the newest of all when the target names a
package set). Its `dependencies` are returned as the ranges they are,
floating. raw.githubusercontent.com/purescript is the public index; a name
that is not a registry name (a git package named by its repository) is not
asked.

## Rationale

The registry has no API of its own: its metadata and the index of manifests
are git repositories.

## Acceptance criteria

1. aff 7.1.0 yields effect and prelude as ranges from the index file alone;
   `>=7.0.0 <8.0.0` asks the metadata and then chooses 7.1.0, not 8.0.0.

## Notes

raw.githubusercontent.com is reachable from the sandbox: `--online
--resolve-depth 1` on purescript-halogen added 29 transitive packages.
