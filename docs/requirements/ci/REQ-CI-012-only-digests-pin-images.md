---
id: REQ-CI-012
uuid: e3af0b73-29ee-445e-b102-fea33aa92c30
title: Only a digest pins a container image
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** parse a container reference as
`[registry/]name[:tag][@digest]`, not reading a registry port as a tag, and
**shall** mark it pinned only when it carries a valid `sha256:` digest, keeping
the tag as the requested version; a reference without a tag **shall** resolve
with no version and be marked floating (REQ-CI-013).

## Rationale

An image tag is republished whenever its owner likes, so `nginx:1.25.3` floats;
only an OCI digest is immutable. No tag at all means whatever `:latest` is
today, the loosest reference there is.

## Acceptance criteria

1. `nginx:1.25.3` resolves to version `1.25.3`, floating.
2. `postgres:16@sha256:<64 hex>` resolves to the digest, requested `16`, pinned.
3. `localhost:5000/app` resolves to package `localhost:5000/app`, with no
   version, floating.
4. `nginx` resolves with no version, floating.
