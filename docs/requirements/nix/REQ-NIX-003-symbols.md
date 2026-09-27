---
id: REQ-NIX-003
uuid: 9ab79f07-1737-4547-860f-8159b5e39528
title: Symbols
scope: nix
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

A Nix file's symbols **shall** be its top-level `let` bindings (`function`
when bound to a lambda, else `var`) and the attributes of the attribute set
it returns, through its function head, `let`, `with`, `assert`, `//` and
both branches of an `if`, their paths cut at two names (`services.nginx`),
as `attr` or `function`. A `flake.nix`'s symbols **shall** be its outputs,
through `flake-utils`-style calls (the last argument of an application), as
paths of up to two names (`packages.x86_64-linux`, `nixosModules.default`,
`packages.default` under `eachDefaultSystem`), of kind `output`.

## Rationale

Top-level attributes are what other files select from a Nix file; a flake's
outputs are what it offers.

## Acceptance criteria

1. The fixture's `lib/default.nix` has `helper`, `version`, `mkThing`,
   `strings.upper` and `quoted`; the dynamic `${...}` attribute is skipped.
2. The fixture's `flake.nix` has `packages.default`, `devShells.default`,
   `nixosModules.default`, `nixosConfigurations.box` and their first names.
