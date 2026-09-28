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

The system **shall** read Cargo tokens from `config.toml`, then `credentials`
(or `credentials.toml`), in `CARGO_HOME` (else `~/.cargo`), then from
`CARGO_REGISTRIES_<NAME>_TOKEN` and `CARGO_REGISTRY_TOKEN`, each over the one
before; **shall** send a token as the whole `Authorization` header value,
exactly as written, as Cargo does; **shall** send a named registry's token
only to the path of the index declared for that name - by
`CARGO_REGISTRIES_<NAME>_INDEX`, else by `config.toml` beside the credentials,
names compared upper-cased with `-` as `_` - whether or not the registry's
`config.json` says `auth-required`; **shall** send the default registry's token
to crates.io only; and **shall** skip the plaintext token of a registry whose
credential provider is not `cargo:token`.

## Rationale

Cargo names registries rather than addressing them; the index URL for each name
is in the configuration beside the token. Cargo puts the token into the
header verbatim, so a registry documents the form it wants: a bare token
(crates.io and most registries) or one with its scheme (Artifactory's
`Bearer <token>`). Adding a scheme of our own sends `Bearer Bearer …` to the
one and a scheme nobody asked for to the other. One host may serve several
registries with different tokens, and crates.io's token is a publishing token
that its public index never needs. A `credential-provider` other than
`cargo:token` (`registry.global-credential-providers` for a registry without
one; `CARGO_REGISTRIES_<NAME>_CREDENTIAL_PROVIDER`,
`CARGO_REGISTRY_CREDENTIAL_PROVIDER` and
`CARGO_REGISTRY_GLOBAL_CREDENTIAL_PROVIDERS` over the files) keeps the
credential in a keychain or a program, which is not run.

## Acceptance criteria

1. A token for registry `corp` is sent as the `Authorization` header, exactly
   as written (`abc` as `abc`, `Bearer abc` as `Bearer abc`), to the path of
   `registries.corp.index`, and not to another path of the same host.
2. A token for a name the configuration does not declare is not kept.
3. The `[registry]` token and `CARGO_REGISTRY_TOKEN` go to crates.io and to no
   other host - not index.crates.io, not a registry replacing crates.io.
4. A sparse registry that requires authentication answers the index requests.
5. A registry whose credential provider is not `cargo:token` gets no token.
