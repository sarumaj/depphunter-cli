---
id: REQ-BEAM-013
title: Hex organization packages named with their repository
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record the Hex repository of a package published to
another repository than hex.pm's public one: `organization: "acme"` (as
`hexpm:acme`) or `repo: "..."` in a `mix.exs` dependency, the repository
`mix.lock` records for the package (`"hexpm:acme"`), and the `repo:` option
of each requirement `mix.lock` lists for a package. hex.pm's public
repository (`hexpm`) is recorded as none.

## Rationale

A private organization's package is served by that organization alone
([REQ-SUP-047](../sup/REQ-SUP-047-hex-api.md)); a package of the same name on
hex.pm is a different package, and may be an attacker's.

## Acceptance criteria

1. `organization: "acme"`, `repo: "hexpm:acme"` and a `mix.lock` entry of
   repository `hexpm:acme` give the package the registry `hexpm:acme`; `repo:
   "hexpm"` and no option give none; another repository name is kept as it is.
2. A locked package's requirements keep the repository `mix.lock` records for
   each.
