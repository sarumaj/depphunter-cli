---
id: REQ-DOCKER-001
uuid: ccd95049-7ddf-403a-b15e-e3fc1f9e62ee
title: Dockerfiles and Compose files claimed
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The docker plugin **shall** analyze, case-insensitively, files named
`Dockerfile` or `Containerfile`, files named after them with a suffix
(`Dockerfile.dev`) or a prefix (`api.Dockerfile`), except ignore files
ending in `.dockerignore`, and Compose files: `compose.yaml`,
`compose.yml`, `compose.*.yaml`/`.yml` and `docker-compose*.yaml`/`.yml`.
Which of the two kinds a file is **shall** be part of its cache key, and every
Dockerfile name **shall** be labelled with the language `Docker`.

## Rationale

Build tools look for these names by default or are pointed at them with
`-f`, and a repository with several images names each Dockerfile after what
it builds. A `Dockerfile.yml` and a `compose.yml` share an extension but
are read by different parsers.

## Acceptance criteria

1. `Dockerfile`, `Containerfile`, `build/Dockerfile.dev`,
   `api.Dockerfile`, `worker.dockerfile`, `compose.yaml`,
   `compose.override.yaml` and `deploy/docker-compose.prod.yaml` are
   claimed.
2. `Dockerfile.dockerignore`, `composer.yml` and `docker/config.yml` are
   not claimed, nor is a binary file.
3. `Dockerfile.yml` and `compose.yml` with the same content have different
   cache keys.
