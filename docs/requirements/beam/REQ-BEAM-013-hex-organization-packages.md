---
id: REQ-BEAM-013
title: Hex organization packages named with their repository
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record the Hex repository of a package published to
another repository than hex.pm's public one: `organization: "acme"` (as
`hexpm:acme`) or `repo: "..."` in a `mix.exs` dependency, the repository
`mix.lock` records for the package (`"hexpm:acme"`), and the `repo:` option
of each requirement `mix.lock` lists for a package. hex.pm's public
repository (`hexpm`) is recorded as none.

rebar3 records no repository for a package: neither a `rebar.config`
dependency (`name`, `{name, "1.0.0"}`, `{name, "1.0.0", {pkg, pkg_name}}`) nor
a `rebar.lock` entry (`{<<"name">>, {pkg, <<"pkg_name">>, <<"1.0.0">>},
Level}`, with its hashes in `pkg_hash`) names one. It asks the repositories its
configuration names, in order - the `{hex, [{repos, [#{name =>
<<"hexpm:acme">>}]}]}` entries of the project's `rebar.config`, then those of
the global one - and then hex.pm's public repository, unless the first `repos`
entry is `{repos, replace, [...]}`, whose list is then the whole. The plugin
**shall** therefore record, for every Hex package of a rebar3 project (locked
or not; not a git or path one), that order: the project's repositories,
comma-separated, followed by `*` for the machine's and hex.pm's, or the
replacing list alone. The repositories of an umbrella's root `rebar.config`
apply to its applications; a Mix project's packages are Mix's (a
`rebar.config` beside a `mix.exs` does not change them).

## Rationale

A private organization's package is served by that organization alone
([REQ-SUP-047](../sup/REQ-SUP-047-hex-api.md)); a package of the same name on
hex.pm is a different package, and may be an attacker's.

## Acceptance criteria

1. `organization: "acme"`, `repo: "hexpm:acme"` and a `mix.lock` entry of
   repository `hexpm:acme` give the package the registry `hexpm:acme`; `repo:
   "hexpm"` and no option give none; another repository name is kept as it is.
2. A locked package's requirements keep the repository `mix.lock` records for
   each.
3. A rebar3 project with `{hex, [{repos, [#{name => <<"hexpm:acme">>}]}]}`
   gives its locked, unlocked and umbrella applications' packages the registry
   `hexpm:acme,*`; one without repositories `*`; `{repos, replace, [acme,
   hexpm]}` gives `hexpm:acme,hexpm` and a replacing `hexpm` alone none; a git
   dependency and a Mix project's packages get none.

## Notes

`rebar.lock` (v1 `{"1.2.0", [...]}` and the older bare list) carries no
repository in any form, so a rebar3 package cannot be pinned to the
organization it came from: it is asked of the repositories in rebar3's order,
as rebar3 itself resolves it
([REQ-SUP-047](../sup/REQ-SUP-047-hex-api.md)). `rebar.config.script` and
profiles' `hex` options are not read.
