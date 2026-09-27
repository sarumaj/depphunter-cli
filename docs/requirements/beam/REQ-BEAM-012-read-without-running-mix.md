---
id: REQ-BEAM-012
uuid: e2d7a761-4f8a-4df2-ad6c-a1671c80afae
title: BEAM read without running Mix or rebar3
scope: beam
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run Mix, rebar3 or the compiler: modules defined by
macros (`embeds_one ... do`, generated route helpers) are found only by their
defined prefix, aliases a package's `__using__` injects are not known, calls
through variables (`apply(mod, ...)`, `mod.fun()`) are not seen, deps declared
in `rebar.config.script`, erlang.mk Makefiles or computed in `mix.exs` are not
read, and `rebar.lock` has no package-to-package edges; private Hex
organizations and repositories (`organization:`, `repo:`) are not asked.

## Rationale

depphunter reads repositories statically and never executes their code.
The vendored tree-sitter grammars were measured first and not used: Elixir
took 4.5 ms per file on average (394 ms for Jason's decoder) and Erlang 5 to
10 ms, and an Erlang macro in a pattern (`?WITH_STACKTRACE(...)`) made ERROR
nodes in 17 of rebar3's 272 files; the lexers read all of these.

## Acceptance criteria

1. With `--resolve-depth 1` and without `--online`, a rebar3 project's
   packages are not walked.
