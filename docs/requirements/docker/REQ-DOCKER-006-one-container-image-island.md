---
id: REQ-DOCKER-006
title: One container-image island shared with CI
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The docker plugin **shall** resolve every image into the ecosystem `oci`
("Container images") that the CI plugin uses, with the same parsing and pinning
rule (REQ-CI-012, REQ-CI-013): only an `@sha256:` digest pins, with the tag
beside it as the requested version; a tag floats; no tag at all is floating
with no version.

## Rationale

A base image in a Dockerfile, a service image in a Compose file and the image a
CI job runs in are the same kind of dependency. One island means one building
per image, whichever file names it, and it means the same private patterns
(`oci:`), registry credentials and base-image following under `--online`
(REQ-SUP-017, REQ-SUP-026) apply to all of them.

## Acceptance criteria

1. `COPY --from=nginx:1.27@sha256:<64 hex>` resolves to the digest, requested
   `1.27`, pinned.
2. `FROM node:20-alpine` resolves to version `20-alpine`, not pinned.
3. The plugin declares the ecosystem `oci` named "Container images".
