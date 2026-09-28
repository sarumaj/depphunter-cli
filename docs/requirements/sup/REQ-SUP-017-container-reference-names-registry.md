---
id: REQ-SUP-017
title: A container reference names its registry
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index of a container image **shall** be the registry its reference names: a
first path segment containing a dot or a port, or `localhost`, is a registry
host; any other reference is a Docker Hub image.

## Rationale

"ghcr.io/org/app" is not "app" from Docker Hub; nothing needs to be configured
to see that.

## Acceptance criteria

1. `nginx` and `library/nginx` resolve from `https://registry-1.docker.io`.
2. `ghcr.io/org/app` resolves from `https://ghcr.io` and `localhost:5000/app`
   from `https://localhost:5000`.

## Notes

An image on a registry other than Docker Hub is marked as coming from an index
nothing here vouches for (see
[REQ-SUP-018](REQ-SUP-018-repository-only-index-marked.md)) unless this
machine's container configuration names that registry (`auths` or
`credHelpers`, or a `[[registry]]` of registries.conf) or `--trust-index` vouches
for it. Mirrors this machine configures are asked before the registry
([REQ-SUP-068](REQ-SUP-068-container-registry-mirrors.md)).
