---
id: REQ-CRYSTAL-008
uuid: d3b0354d-9b8a-4bf7-8115-0a501c33de6d
title: Installed shards' dependencies
scope: crystal
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read the shards installed in `lib/` (directories and
the symlinks shards makes for path dependencies) and the `shard.yml` each
ships; `--resolve-depth` **shall** follow those `dependencies` (not
development dependencies), each pinned as the installing project's lock or
manifest names it, and report them as installed. Without `lib/` nothing is
known offline: `shard.lock` is flat.

## Rationale

shards records no dependency edges in its lock; the installed shards'
manifests are the only offline source.

## Acceptance criteria

1. The installed kemal depends on exception_page (locked 0.4.1) and radix
   (the override's commit), not on its development dependency ameba; db,
   not installed, has no known dependencies.
