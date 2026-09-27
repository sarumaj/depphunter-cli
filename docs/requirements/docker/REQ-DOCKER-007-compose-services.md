---
id: REQ-DOCKER-007
title: Compose services' images and builds
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

For each service of a Compose file the docker plugin **shall** report: when the
service has no `build`, its `image` as a container image; when it has a
`build` (a context string, or `context` and `dockerfile`, defaulting to
`.` and `Dockerfile`), an edge to that Dockerfile, read from the Compose
file's directory, when the repository has it, and not its `image`, which only
names the result; the images of an inline Dockerfile
(`dockerfile_inline`); and the `docker-image://` values of
`additional_contexts`. A remote build context **shall** be ignored. Merge
keys (`<<: *anchor`) **shall** be followed. Services **shall** be the file's
symbols, of kind `service`.

## Rationale

A service that is built depends on its Dockerfile, whose own base images chain
on from there; one that is pulled depends on its image. The image name of a
built service is a tag the build writes, not something pulled.

## Acceptance criteria

1. `build: .` resolves to the local `Dockerfile`; the service's `image:`
   is not reported.
2. `context: ./api` with `dockerfile: api.Dockerfile` resolves to
   `api/api.Dockerfile`; `context: ..` in `deploy/` resolves from the
   repository root.
3. `base: docker-image://alpine:3.20` yields `alpine`.
4. `build: ./nowhere` with no Dockerfile there, and a Git URL context, yield
   no edge.
5. An image shared through a merge key is reported for the service using it.
