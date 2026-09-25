---
id: REQ-CI-007
uuid: 475bffa4-ba5b-4ee4-aadd-7ea47f669540
title: Images and services of GitLab pipelines
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** record as container-image dependencies the `image:` and
`services:` of a GitLab pipeline at top level, under `default:` and in every
job, in string form or as `name:` of a mapping, and **shall** skip an image
reference containing a variable (`$`).

## Rationale

The images a pipeline runs in are third-party code; a reference built from a
variable cannot be expanded offline.

## Acceptance criteria

1. `image: node:20` resolves to OCI package `node`, version `20`.
2. A `default:` service `name: redis:7.2` resolves to OCI package `redis`,
   version `7.2`.
3. A job `image: { name: python:3.12-slim }` resolves to OCI package `python`,
   version `3.12-slim`.
