---
id: REQ-GLEAM-009
title: One Hex island with Elixir and Erlang
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

Gleam packages **shall** be packages of the BEAM plugin's `hex` island, named
by their Hex name, so that a package reached from Gleam, Elixir or Erlang is
one node, is asked about in OSV's `Hex` ecosystem, and is followed by the
hex.pm client of `--online`. The BEAM plugin **shall** resolve an Erlang or
Elixir reference to a compiled Gleam module (`gleam@list`,
`:gleam@list`) to the project's `.gleam` file or the package the Gleam
plugin names it by.

## Rationale

Gleam compiles to the BEAM and publishes to Hex; mix and rebar3 install
Gleam packages too.

## Acceptance criteria

1. An Elixir project whose `mix.lock` locks gleam_stdlib 0.40.0 beside a Gleam
   package whose manifest locks it has one `hex` node `gleam_stdlib`, with
   edges from `mix.exs`, an Elixir file calling `:gleam@list`, `gleam.toml`,
   `manifest.toml` and a module importing `gleam/list`.
2. An Erlang FFI file calling `core@util:x()` has an edge to
   `src/core/util.gleam`.
