---
id: REQ-AUTH-008
title: Cargo registry tokens
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read Cargo tokens from `credentials` (or
`credentials.toml`) in `CARGO_HOME` (else `~/.cargo`) and from
`CARGO_REGISTRIES_<NAME>_TOKEN` and `CARGO_REGISTRY_TOKEN`, and **shall** file
a named registry's token under the host of the index declared for that name -
by `CARGO_REGISTRIES_<NAME>_INDEX`, else by `config.toml` beside the
credentials, names compared upper-cased with `-` as `_` - and the default
registry's under crates.io.

## Rationale

Cargo names registries rather than addressing them; the index URL for each name
is in the configuration beside the token.

## Acceptance criteria

1. A token for registry `corp` is sent as a Bearer credential to the host of
   `registries.corp.index`.
2. A token for a name the configuration does not declare is not kept.
