---
id: REQ-CRYSTAL-007
uuid: 45f2d042-78b1-42b0-a960-58baefa3342e
title: Requires resolved to shards and the standard library
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A require by name **shall** resolve, in the compiler's order: to the shard
installed in the `lib/` of the file's project or the repository's root
(the directory's name is the shard), then to the project's own files
(REQ-CRYSTAL-004), then to a shard the project's `shard.yml`, `shard.lock`
or `shard.override.yml` names exactly, then to the standard library by its
first segment (`json`, `http/client` is `http`, `digest/sha256` is
`digest`, `c/stdio` is `lib_c`), then to a declared shard whose name matches
the first segment with `-`/`_`, a `crystal-` prefix and a `.cr` suffix
folded (`sqlite3` is `crystal-sqlite3`), else unresolved, named by its
first segment. A file no `shard.yml` governs **shall** look through every
project's shards, shallowest first; a require of the project's own name that
names no file is dropped.

## Rationale

`lib/` is authoritative when it exists; otherwise the manifests say which
shards the project has, and a require's first segment is the shard's name
by convention.

## Acceptance criteria

1. `kemal/cli` is kemal (installed), `exception_page` the shard only the
   lock names, `sqlite3` crystal-sqlite3, `json` and `digest/sha256` the
   standard library, `nothere/thing` an unresolved `nothere`.
