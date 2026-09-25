---
id: REQ-CI-004
uuid: 58bba5ab-80c0-4697-919c-a2cdf27b0bba
title: Container images of GitHub workflows and actions
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** record as container-image dependencies a job's
`container:` (as a string or as `image:` of a mapping), the image of every entry
of a job's `services:`, every step `uses: docker://<image>`, and a Docker
action's `runs.image` (with or without `docker://`, except a `Dockerfile`).

## Rationale

The images a workflow runs in are third-party code as much as its actions are.

## Acceptance criteria

1. `container: golang:1.27-alpine` resolves to OCI package `golang`, version
   `1.27-alpine`.
2. A service `postgres: postgres:16@sha256:…` resolves to OCI package `postgres`
   at the digest.
3. `uses: docker://alpine:3.19` resolves to OCI package `alpine`, version
   `3.19`.
4. An action with `runs.image: docker://alpine:3.19` records that image; one
   with `runs.image: Dockerfile` records none.

## Notes

Criterion 4 has no automated test.
