---
id: REQ-BEAM-007
uuid: 101b282a-a733-4880-aeb2-b277afad8331
title: Elixir and OTP islands
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** put Elixir's own modules (`Kernel`, `Enum`,
`GenServer`, `Logger`, `ExUnit`, `Mix`, ... by first segment, and the
compiler's `elixir_*` Erlang modules as `elixir`) in a hidden `elixir-std`
island, and Erlang/OTP's modules (by name and family prefix) and applications
(`-include_lib("kernel/...")`, application lists) in a hidden `erlang-std`
island, unless the project defines the module outside test code.

## Rationale

Standard libraries are hidden unless asked for, as for every language.

## Acceptance criteria

1. `use GenServer` goes to elixir-std `GenServer`, `:ets.lookup` to
   erlang-std `ets`, `-include_lib("kernel/include/logger.hrl")` to erlang-std
   `kernel`.
