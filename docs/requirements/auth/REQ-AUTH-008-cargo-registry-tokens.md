---
id: REQ-AUTH-008
uuid: 0066273e-0ee1-4fee-8f73-af5a6047d681
title: Cargo registry tokens
scope: auth
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The system **shall** read Cargo tokens from `~/.cargo/credentials.toml` (or
`credentials`) and from `CARGO_REGISTRIES_<NAME>_TOKEN` and
`CARGO_REGISTRY_TOKEN`, and **shall** file a named registry's token under the
host of the index `~/.cargo/config.toml` declares for that name, and the default
registry's under crates.io.

## Rationale

Cargo names registries rather than addressing them; the index URL for each name
is in the configuration beside the token.

## Acceptance criteria

1. A token for registry `corp` is sent as a Bearer credential to the host of
   `registries.corp.index`.
2. A token for a name the configuration does not declare is not kept.
