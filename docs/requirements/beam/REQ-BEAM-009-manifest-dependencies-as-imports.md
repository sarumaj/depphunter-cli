---
id: REQ-BEAM-009
uuid: 1b133614-6e69-4812-b454-e80b94b85a09
title: Manifest dependencies as imports
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** make each dependency of a `mix.exs` (the list its
`deps` function returns, or `deps: [...]` in the project) and of a
`rebar.config` (`deps`, and those of its profiles) an import of what it
declares - a path or `in_umbrella` dependency the application's manifest in
the repository, anything else its package - and each application an
`.app.src` lists in `applications` or `included_applications` an import of
that application: the repository's, Elixir's or OTP's, or a package.

## Rationale

A dependency used only at run time (an adapter, a server) has no module
reference to show it otherwise.

## Acceptance criteria

1. `{:shop, in_umbrella: true}` resolves to `apps/shop/mix.exs` and
   `{:local_lib, path: "../../libs/local_lib"}` to `libs/local_lib/mix.exs`.
