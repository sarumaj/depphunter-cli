---
id: REQ-CRYSTAL-005
uuid: 96b4ee62-b4ee-4cc8-95fd-5c4ebeb9a823
title: shard.yml and shard.override.yml
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every dependency and development dependency of a `shard.yml` **shall** be
an import of the shard it names, with its source (`github:`, `gitlab:`,
`bitbucket:`, `codeberg:`, `git:` or `path:`) and requirement (`version:`,
`branch:`, `tag:` or `commit:`); every target's `main:` file **shall** be an
import of that file. A `shard.override.yml` beside it (read from disk when
not scanned) **shall** replace the entries of the dependencies it names,
and each of its entries is an import too. A `path:` dependency is an edge to
its directory, dropped when that is not in the repository.

## Rationale

Applications load their shards by `require`, but a manifest's
dependencies are the project's supply chain even when no file requires
them.

## Acceptance criteria

1. The fixture's `shard.yml` imports kemal, db, radix, markd, crinja,
   internal, crystal-sqlite3, spectator and the `libs/widgets` directory,
   and `src/shop.cr` through `main:`; radix is the override's commit.
