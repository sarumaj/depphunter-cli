---
id: REQ-BEAM-004
title: Erlang attributes and calls read
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

<!-- cSpell: words behaviour -->
The plugin **shall** read from an Erlang file its `-include` and
`-include_lib` paths, `-behaviour` (or `-behavior`) and `-import` modules, and
every remote call, fun reference or remote type `mod:fun` whose module is an
atom - not a variable, a macro (`?MODULE:f()`), the module itself or the class
of a catch clause (`throw:not_found`). A `-include` resolves beside the file,
in its application's `include/` or `src/`, at the application root or in a
local application its path starts with, and is dropped when none has it.

## Rationale

These are all the ways an Erlang module depends on another one statically.

## Acceptance criteria

1. `-include("erl_app.hrl")` in `erl/src/erl_app.erl` resolves to
   `erl/include/erl_app.hrl`; `-include("generated.hrl")` is dropped.
