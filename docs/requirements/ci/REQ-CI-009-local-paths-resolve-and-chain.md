---
id: REQ-CI-009
uuid: 16712378-74fb-4186-bad5-1f87781d3b0a
title: Local CI paths resolve inside the repository
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** resolve a local reference (`./path`, or a GitLab `local`
include) from the repository root, whichever directory the referring file is in,
to the file it names; a reference to a directory holding an `action.yml` or
`action.yaml` **shall** resolve to that file, so that a local action's own
dependencies chain on; a path leaving the repository **shall** resolve to
nothing.

## Rationale

Both platforms read such paths from the repository root. A composite action is
named by its directory, but its dependencies are written in the `action.yml`
inside it.

## Acceptance criteria

1. `uses: ./.github/actions/setup` resolves to
   `.github/actions/setup/action.yml`, whose own `uses:` are dependencies of
   that file.
2. `include: local: /.gitlab/ci/build.yml` resolves to `.gitlab/ci/build.yml`.
