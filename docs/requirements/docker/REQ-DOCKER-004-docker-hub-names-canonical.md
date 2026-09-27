---
id: REQ-DOCKER-004
uuid: c6250cda-cff8-419d-a407-e66a780fab73
title: Docker Hub image names canonical
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every container reference, whether a Dockerfile, a Compose file or a CI
pipeline names it, **shall** name a Docker Hub image by its short name: the
registry `docker.io`, `index.docker.io` or `registry-1.docker.io` and the
namespace `library/` of an official image **shall** be dropped. A name on any
other registry **shall** be kept as written.

## Rationale

`docker.io/library/nginx`, `library/nginx` and `nginx` pull the same
image and should be one package on the map. The short name is also the one the
index client reads as Docker Hub (REQ-SUP-017), so a long one would otherwise
be taken for an unknown registry under `--online`.

## Acceptance criteria

1. `docker.io/library/nginx:1.27`, `index.docker.io/library/nginx`,
   `registry-1.docker.io/nginx` and `library/nginx` resolve to `nginx`.
2. `docker.io/bitnami/redis:7` resolves to `bitnami/redis`.
3. `ghcr.io/library/tool` keeps its name.
