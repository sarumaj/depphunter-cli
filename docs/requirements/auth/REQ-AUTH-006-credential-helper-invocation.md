---
id: REQ-AUTH-006
title: Container credential helpers
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

For a registry that the container configuration assigns to a helper
(`credHelpers`, or `credsStore` for the registries under `auths`) and for which
no credential is stored, the system **shall** run `docker-credential-<name> get`
with the registry on standard input and use the username and secret it prints.

## Rationale

Most installations no longer store a credential; they name a helper that must be
asked, as `docker login` does.

## Acceptance criteria

1. A helper found on PATH that prints a credential for the registry supplies
   that registry's credential.
2. A helper is run only for a registry the configuration names.
