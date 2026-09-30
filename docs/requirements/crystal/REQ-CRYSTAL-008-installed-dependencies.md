---
id: REQ-CRYSTAL-008
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
the symlinks shards makes for path dependencies, as long as they stay inside
the repository: see REQ-LANG-031) and the `shard.yml` each ships;
`--resolve-depth` **shall** follow those `dependencies` (not development
dependencies), each pinned as the installing project's lock or manifest
names it, and report them as installed. `lib/.shards.info` (the
versions shards installed, in `shard.lock`'s format) **shall** pin any shard
the project's `shard.lock` does not name. Without `lib/` nothing is known
offline: `shard.lock` is flat.

## Rationale

shards records no dependency edges in its lock; the installed shards'
manifests are the only offline source.

## Acceptance criteria

1. The installed kemal depends on exception_page (locked 0.4.1) and radix
   (the override's commit), not on its development dependency ameba; db,
   not installed, has no known dependencies.
2. Without a `shard.lock`, `lib/.shards.info` pins kemal at 1.4.0
   (requested `~> 1.4`) and kemal's dependency radix at 0.4.1; with a
   garbage `.shards.info` radix floats on kemal's `~> 0.4.0`.
