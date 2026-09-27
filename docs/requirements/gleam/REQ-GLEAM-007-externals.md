---
id: REQ-GLEAM-007
title: External functions resolved
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** read `@external(erlang, "mod", "f")` as an
import of the Erlang module: a compiled Gleam module (`gleam@list`) as that
Gleam module, a project `.erl` file, a declared or locked package of that
name or Erlang application (`otp_app`), Erlang/OTP (the BEAM plugin's
erlang-std island), a package its prefix names (`gleam_otp_external` is
`gleam_otp`), else an unresolved Hex package of that name; an Elixir module
is dropped. `@external(javascript, "path", "f")` **shall** resolve a
relative path from the module's place in the compiled output, which mirrors
`src/`: a file of the package, or, when it climbs out of the package, the
package it names (`../../gleam_stdlib/gleam/list.mjs`); a bare specifier is
an npm package, pinned by an exact version in the package's `package.json`
or unresolved. Pre-0.30 `external fn ... = "mod" "f"` is read the same way.

## Rationale

Externals are how Gleam code reaches Erlang and JavaScript.

## Acceptance criteria

1. In `src/shop/ffi.gleam`, `shop_ffi` is `src/shop_ffi.erl`, `os` is
   erlang-std, `hpack` is hpack_erl by its `otp_app`, and `react` is npm
   react 18.3.1.
2. `../shop_ffi.mjs` from `src/shop/cart.gleam` is `src/shop_ffi.mjs`.
