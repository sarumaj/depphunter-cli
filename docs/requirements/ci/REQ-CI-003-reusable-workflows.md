---
id: REQ-CI-003
uuid: bb68cb7d-f25a-4686-bb79-6c83c779c887
title: Reusable workflows
scope: ci
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The CI plugin **shall** record a job-level `uses:` (a reusable workflow) as a
dependency: on the workflow file when it is a local path, and otherwise on the
repository that holds it.

## Rationale

A job that calls a reusable workflow has no steps of its own; the called
workflow is what it runs.

## Acceptance criteria

1. `uses: octo-org/shared/.github/workflows/release.yml@<commit>` resolves to
   package `octo-org/shared`, pinned.
2. `uses: ./.github/workflows/build.yml` resolves to
   `.github/workflows/build.yml`.
