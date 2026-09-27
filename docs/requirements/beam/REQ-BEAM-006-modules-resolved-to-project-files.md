---
id: REQ-BEAM-006
title: Modules resolved to project files
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve an Elixir module to the project file that
defines it (an index of every `defmodule` and `defprotocol` in the repository,
umbrella applications included, where a definition outside test code wins),
else - once Elixir's own modules, curated package prefixes and fetched
packages are ruled out - to the file defining its longest defined prefix
(`MyAppWeb.Router.Helpers` is generated in `MyAppWeb.Router`); an Erlang module
to `<module>.erl` (or the `.xrl`/`.yrl` it is generated from), and one naming a
compiled Gleam module (`gleam@list`) as REQ-GLEAM-009 says; a reference
whose first segment the file does not alias, through the aliases that the
quote blocks of the modules it uses inject (`use MyApp.Schema`,
`use MyAppWeb, :controller`), following their own uses. A reference in the
project's own namespace that nothing defines is dropped, and so is one that
only a package's `__using__` can have aliased.

## Rationale

Elixir does not tie a module to a path, so only an index of declarations
finds it; Erlang does.

## Acceptance criteria

1. After `use Shop.Schema`, whose quote aliases `Ecto.Multi` and `Shop.Cart`,
   `Multi` resolves to ecto and `Cart` to `apps/shop/lib/shop/cart.ex`.
2. `:legacy_parser.parse` in Elixir resolves to
   `apps/shop_web/src/legacy_parser.erl`, and `'Elixir.Shop.Cart':new` in
   Erlang to `apps/shop/lib/shop/cart.ex`.
