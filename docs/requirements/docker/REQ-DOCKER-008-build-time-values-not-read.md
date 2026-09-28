---
id: REQ-DOCKER-008
title: Build-time values and remote Compose files not read
scope: docker
type: limitation
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The docker plugin **shall not** read values supplied outside the repository:
`--build-arg`, the environment, `args:` of a Compose build, an `--env-file`,
an `env_file:` of an `include:` entry, or an `.env` file other than the one
beside the Compose file (REQ-DOCKER-003) - Compose's project directory, the
first `-f` file's, is not known, so an override file in another directory
takes its own directory's. It **shall not** follow a remote `include:`
(`oci://`, a Git URL), a Compose file outside the repository, or a remote
build context.

## Rationale

What a build argument or the environment sets differs between machines and CI
runs, and a remote Compose file or context is not in the repository. A
reference using such a value is kept as written (REQ-DOCKER-003) rather than
guessed.

## Acceptance criteria

1. Inspection of internal/lang/docker shows no reading of the environment or
   Compose `args:`, and `.env` files read only in the resolver
   (`resolver.env`).
