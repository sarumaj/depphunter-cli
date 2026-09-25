---
id: REQ-SUP-017
uuid: 2eeb80a2-23d4-42c0-9fe3-ae2eafc37578
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

An image on any registry other than Docker Hub is marked as coming from an index
nothing here vouches for (see
[REQ-SUP-018](REQ-SUP-018-repository-only-index-marked.md)); `--trust-index`
does not change that for container registries.
