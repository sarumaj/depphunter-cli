---
id: REQ-DOCKER-008
uuid: 5460db48-e975-419c-8ad6-264e56de126d
title: Build-time values and remote Compose files not read
scope: docker
type: limitation
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The docker plugin **shall not** read values supplied outside the file that
uses them: `--build-arg`, the environment, a Compose `.env` file or
`args:` of a Compose build. It **shall not** follow a Compose `include:`,
an `extends:` in another file, or a remote build context.

## Rationale

What a build argument or an `.env` file sets differs between machines and CI
runs, and an extraction depends only on the file's own content. A reference
using such a value is kept as written (REQ-DOCKER-003) rather than guessed.

## Acceptance criteria

1. Inspection of internal/lang/docker shows no reading of `.env` files, the
   environment or Compose `args:`.
