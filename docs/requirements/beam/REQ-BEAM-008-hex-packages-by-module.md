---
id: REQ-BEAM-008
uuid: 6ec7132a-d6df-4c40-835a-715c6a25f27d
title: Hex packages found by module name
scope: beam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** attribute a module that is not the project's to a Hex
package: the package that defines it in `deps/<app>` or
`_build/*/lib/<app>/ebin` on disk; a curated prefix table when the project
knows that package (`Ecto.Adapters.SQL` -> ecto_sql, `Bcrypt` ->
bcrypt_elixir); else the longest prefix whose segments, joined and folded
(lower case, no underscores), equal a package the manifests declare or the lock
holds (`Phoenix.LiveView` -> phoenix_live_view), or the namespace joined with
the last segment (`Ueberauth.Strategy.Github` -> ueberauth_github); an Erlang
module to a declared package of its name or of a prefix before an underscore
(`cowboy_req` -> cowboy). Anything else is an unresolved package named after
the underscored first segment (Elixir) or the module (Erlang).

## Rationale

A module does not say which package ships it; the lock lists every package
installed, transitive ones included, and packages name their modules after
themselves.

## Acceptance criteria

1. `UUID` resolves to elixir_uuid through `deps/elixir_uuid/lib/uuid.ex`, and
   `ec_file` to erlware_commons through its `_build` module file.
2. `Unknown.Thing` is an unresolved package `unknown`.
