---
id: REQ-BEAM-003
title: Elixir definitions extracted
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract Elixir modules (`defmodule`, qualified by the
enclosing module), protocols (`defprotocol`), implementations (`defimpl`, as
`Proto.Type`), structs and exceptions (`%Mod{}`), types (`@type`, `@typep`,
`@opaque` as `Mod.name`) and the functions defined directly in a module body as
`Mod.fun/arity` - `def`, `defdelegate` and `defn` of kind function, `defp` and
`defnp` func, `defmacro`, `defmacrop`, `defguard` and `defguardp` macro - once
per name and arity.

## Rationale

Arity is part of an Elixir function's identity; clauses of one function are
one symbol.

## Acceptance criteria

1. `def start(name)` and two clauses of `def start(name, opts)` give
   `Shop.start/1` and `Shop.start/2`.
2. A nested `defmodule Error` in `Shop` gives `Shop.Error`, and its
   `defexception` `%Shop.Error{}`.
