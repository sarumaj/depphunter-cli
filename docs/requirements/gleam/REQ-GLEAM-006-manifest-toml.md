---
id: REQ-GLEAM-006
uuid: 79a788bf-29dd-48e5-a8df-f9f296f3aedf
title: manifest.toml read
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** read the `manifest.toml` beside a `gleam.toml`
(from disk when git ignores it): each package (`name`, `version`, `source`
`hex`, `git` with `repo` and `commit`, or `local` with `path`) is an import
of the manifest, pins what the package's modules and `gleam.toml` resolve to,
and its `requirements` are what `--resolve-depth` follows offline, pinned by
the same manifest. The `[requirements]` table stands in for `gleam.toml`
when giving the requested version.

## Rationale

The manifest is Gleam's lock file: every package the build uses, with its
version and what it requires.

## Acceptance criteria

1. `mist` 1.2.0 depends on gleam_erlang, gleam_http, gleam_otp,
   gleam_stdlib, glisten and hpack_erl at their locked versions.
2. The manifest's `inventory` (source `local`) is an edge to
   `libs/inventory/gleam.toml`.
