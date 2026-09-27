---
id: REQ-SUP-047
uuid: abe77337-c8c2-47a0-b3a1-f6efdf2fdeac
title: Hex dependencies from the Hex API
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Hex package's dependencies from the Hex API
(`https://hex.pm/api` unless `HEX_API_URL` names another): the package
(`<api>/packages/<name>`) for its releases and latest stable release, then the
release asked for, or else that one (`<api>/packages/<name>/releases/<v>`), its
non-optional requirements keyed by package name.

## Rationale

`rebar.lock` records no edges and a library commits no lock, so what a Hex
package depends on is only on hex.pm; the repository mirrors `HEX_MIRROR` names
serve signed protobuf files, not this API.

## Acceptance criteria

1. Against a stub API, plug 1.15.0 depends on `mime` `~> 1.0 or ~> 2.0` and
   `plug_crypto` 2.0.0 (pinned).
2. A requirement names no release, so the latest stable release's requirements
   are returned, without the optional `jason`.
