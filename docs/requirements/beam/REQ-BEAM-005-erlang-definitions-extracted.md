---
id: REQ-BEAM-005
title: Erlang definitions extracted
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract an Erlang file's `-module`, its functions as
`name/arity` (kind function when exported, by `-export` or
`-compile(export_all)`, func otherwise), records as `#name{}`, macros as
`?NAME` and types (`-type`, `-opaque`) as `name()`.

## Rationale

A function's clauses are one function; export decides whether other modules
may call it.

## Acceptance criteria

1. `stop/1` with two clauses is one symbol; an unexported `helper/2` is of
   kind func.
