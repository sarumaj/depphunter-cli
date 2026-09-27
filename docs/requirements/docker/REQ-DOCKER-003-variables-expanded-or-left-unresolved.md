---
id: REQ-DOCKER-003
uuid: 3f2c8db1-1678-4994-a29b-6b3ade67cfe7
title: Build arguments expanded, unknown values left unresolved
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The docker plugin **shall** expand `$NAME`, `${NAME}`, `${NAME:-word}`,
`${NAME-word}`, `${NAME:+word}` and `${NAME+word}` in an image
reference: in a `FROM` from the defaults of the `ARG`s declared before the
first `FROM`, in a `COPY --from` or `RUN --mount` from the `ARG`s of the
stage, and in a Compose file from the defaults written in the reference, with
`\$` (Dockerfile) and `$$` (Compose) as a literal dollar sign. A variable
with no value in the file **shall** be left as written: when it is in the image
name the reference **shall** resolve as unresolved under the reference as
written; when it is only in the tag or digest the image **shall** resolve with
that text as its version, not pinned.

## Rationale

`ARG BASE=node:20` / `FROM ${BASE}` is the common way to make a base
image configurable, and its default is what the file builds. A value only
`--build-arg`, the environment or an `.env` file supplies is not in the
repository; inventing an image for it would put a dependency on the map that
may not exist.

## Acceptance criteria

1. `ARG GO_VERSION=1.22` / `FROM golang:${GO_VERSION}` resolves to
   `golang` `1.22`.
2. `ARG REGISTRY` / `FROM ${REGISTRY}/tools:1` resolves to an unresolved
   package `${REGISTRY}/tools:1`.
3. `FROM python:${TAG}` with `TAG` declared without a default resolves to
   `python` at version `${TAG}`, not pinned.
4. Compose `image: "postgres:${PG_VERSION:-16}"` resolves to `postgres`
   `16`; `image: ${PROXY_IMAGE}` is unresolved.
