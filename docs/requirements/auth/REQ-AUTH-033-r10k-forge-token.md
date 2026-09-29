---
id: REQ-AUTH-033
title: r10k's Forge authorization token
scope: auth
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

`forge: authorization_token` of r10k's configuration
(`/etc/puppetlabs/r10k/r10k.yaml`, else `/etc/r10k.yaml`, the first that
exists) **shall** be sent as the
Authorization header of requests below `forge: baseurl` (the public Forge
without one), as r10k sends it: verbatim (`Bearer <token>`), a value without a
scheme as a Bearer token, over https only.

## Rationale

A private Forge answers only to the token r10k is configured with.

## Acceptance criteria

1. `Bearer secret` reaches forge.corp.example/api's paths and no other; a bare
   token without a baseurl reaches the public Forge.
