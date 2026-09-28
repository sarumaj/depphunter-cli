---
id: REQ-DOCKER-009
title: Compose include and extends
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A Compose service that `extends:` another service (the short form naming a
service of the same file, or `service:` with an optional `file:`) **shall**
take the `image:` and `build:` it does not set itself from that service,
following the chain (up to 8 steps); a `build:` taken from another file
**shall** be read from that file's directory, and the imports taken **shall**
be reported on the `extends:` line. Each file of the chain **shall** be an
edge (`extends: <file>`), dropped when the repository lacks it. Each path of
the top-level `include:` (a path, or `path:` holding one or a list) **shall**
be an edge to that file, relative to the Compose file's directory; an
included file not named as a Compose file (REQ-DOCKER-001) **shall** be read
for the including file, its imports reported on the `include:` line as
`<spec> (<file>)` and resolved with the included file's own directory and
`.env` file. Paths **shall** be interpolated with the `.env` file of the
Compose file's directory (REQ-DOCKER-003); a remote or absolute path, and a
path outside the repository, **shall not** be followed.

## Rationale

Larger Compose projects split services over several files: a base file other
files `extends:`, one file per component that `include:` brings together.
Without them a service's image or Dockerfile is missing from the file that
runs it.

## Acceptance criteria

1. `extends: base` in the same file gives the service base's image, on its
   `extends:` line; a service's own `image:` stays.
2. `extends: {file: common/services.yml, service: builder}` with
   `build: ./ctx` there is an edge to `common/services.yml` and a build of
   `common/ctx/Dockerfile`; a second hop through `../base/more.yml` is read
   from `common/`.
3. `include: [infra/db.yml]` is an edge, and its `postgres:${PG:-16}` is
   `postgres` at the version `infra/.env` sets; `include:` of
   `svc/compose.yaml` is an edge only; `${INFRA_DIR}/cache.yml` is
   interpolated; `oci://`, `../outside.yml` and a missing file give no edge.
4. A garbage `.env` or included file gives no panic and no other imports.
