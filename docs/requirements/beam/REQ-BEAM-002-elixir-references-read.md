---
id: REQ-BEAM-002
title: Elixir module references read
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read from an Elixir file the modules it names in
`alias`, `import`, `require` and `use` (including `alias Foo.{A, B.C}`,
`alias Foo, as: B`, `require Foo, as: B` and `__MODULE__`-relative names) and
in any other reference - a remote call, a struct, a behavior, a `defimpl`
protocol, an argument - expanded through the aliases in effect (a nested
`defmodule` aliases its first segment); the Erlang modules it calls
(`:ets.new`) or takes as a behavior; and, inside a Phoenix router's
`scope "/path", Alias do` block, the controllers relative to the scope alias.
Strings, heredocs, sigils, character literals, comments and interpolations are
not read as code, and a module the file itself defines is not an import.

## Rationale

In Elixir every capitalized name is a module, so a reference is a
dependency whether or not an `alias` or `import` precedes it.

## Acceptance criteria

1. `alias Shop.{Item, Pricing.Rules}` imports `Shop.Item` and
   `Shop.Pricing.Rules`, and `Rules.apply` then names `Shop.Pricing.Rules`.
2. `~s(Decimal.new(1))`, `"#{Fake.x()}"` and a `@moduledoc` heredoc import
   nothing.
