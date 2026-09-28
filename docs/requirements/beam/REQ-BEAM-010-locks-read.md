---
id: REQ-BEAM-010
title: mix.lock and rebar.lock read
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `mix.lock` (`{:hex, :pkg, version, ...}` with its
requirements, `{:git, url, sha, opts}`) and `rebar.lock` (version 1 and the
older bare list; `{pkg, Name, Version}` and `{git, Url, {ref, Sha}}`) from
disk beside every `mix.exs` and `rebar.config`, a project without one using
the enclosing project's (an umbrella's), and answer `--resolve-depth` from the
requirements `mix.lock` records for each Hex package; `rebar.lock` records only
a depth, no edges. A `rebar.lock` without a `mix.lock` beside it **shall** be
noted as flat in the resolution report, and a lock read from disk because the
scan left it out as such
([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)).

## Rationale

Libraries git-ignore their lock; what is on disk is what was resolved.

## Acceptance criteria

1. ecto 3.11.2 depends on decimal 2.1.1 (requested `~> 2.0`), jason and
   telemetry as the lock resolved them; its optional poison, which the lock
   does not hold, is left out.
