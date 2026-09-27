---
id: REQ-CRYSTAL-001
uuid: ee652ba1-2fe0-4fcb-86c0-f1586b4f0680
title: Crystal files claimed
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Crystal plugin **shall** claim Crystal sources (`.cr`) and the shards
dependency manager's `shard.yml`, `shard.lock` and `shard.override.yml`,
telling them apart from other YAML and lock files by their names, and
**shall not** claim anything under the compiler's `.crystal/` cache or
under a `lib/` directory that has a `shard.yml` beside it, where
`shards install` puts the dependencies. A `lib/` directory without a
`shard.yml` beside it is claimed as source. The scanner **shall** label
`.cr` files Crystal and `shard.lock` YAML.

## Rationale

`lib/` is where shards installs every dependency, but many ecosystems keep
their own sources in a `lib/` directory; only the manifest beside it says
which it is.

## Acceptance criteria

1. `src/shop.cr`, `tools/lib/helper.cr`, `shard.lock` and
   `shard.override.yml` are claimed; `lib/kemal/src/kemal.cr` and
   `lib/kemal/shard.yml` beside `shard.yml`, and `.crystal/cache/macro.cr`,
   are not.
2. `shard.yml`, `shard.lock` and `shard.override.yml` have their own
   classes; `config.yml` is not claimed.
