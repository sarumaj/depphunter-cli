---
id: REQ-DOCKER-005
uuid: 5fd27c14-6047-4689-8dad-a1894ba6b544
title: Images of a Dockerfile, stages told apart
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The docker plugin **shall** report as images the frontend a `# syntax=`
directive names and the image of every `FROM` (after its flags, such as
`--platform`), `COPY --from=` and `RUN --mount=…,from=` that names neither
an earlier stage of the file, by name (case-insensitively) or by index, nor
`scratch`. Named stages (`AS name`) **shall** be the file's symbols, of
kind `stage`.

## Rationale

A reference to an earlier stage is internal to the build; `scratch` is the
empty image, not something pulled. Everything else is pulled from a registry
and becomes part of what is built.

## Acceptance criteria

1. `# syntax=docker/dockerfile:1.7` yields `docker/dockerfile` `1.7`.
2. `FROM --platform=$BUILDPLATFORM golang:1.22 AS Build` yields `golang`
   and the symbol `Build`.
3. `COPY --from=build`, `COPY --from=0`, `COPY --from=Runtime` after
   `AS runtime`, `FROM web` after `AS web` and `FROM scratch` yield no
   import.
4. `RUN --mount=type=bind,from=alpine:3.20,…` and `ONBUILD COPY
   --from=busybox:1.36` yield `alpine` and `busybox`.
