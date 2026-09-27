---
id: REQ-BEAM-001
title: Elixir, Erlang and their manifests claimed
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The BEAM plugin **shall** claim Elixir sources (`.ex`, `.exs`, `mix.exs`
included), Erlang sources and headers (`.erl`, `.hrl`), OTP application
resource files (`.app.src`) and `rebar.config`, except files under a
`deps/<app>` or `_build` directory, where Mix and rebar3 put what they fetch and
build.

## Rationale

Elixir and Erlang call each other's modules and share the Hex packages and
the lock files, so one plugin reads both, as the C and C++ plugin does.

## Acceptance criteria

1. `lib/a.ex`, `mix.exs`, `src/a.erl`, `include/a.hrl`, `src/a.app.src` and
   `rebar.config` are claimed; `mix.lock`, `deps/plug/lib/plug.ex` and
   `_build/dev/lib/a/ebin/a.app` are not; `lib/app/deps/helper.ex` is.
