---
id: REQ-CI-002
uuid: 7b63621c-5ce3-4fb6-b60e-73ed0077c7fd
title: Workflow and composite action steps
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** record the `uses:` of every step of every job of a
GitHub workflow, and of every step of a composite action's `runs.steps`, as a
dependency; a reference `owner/repo[/path][@ref]` **shall** resolve to the
package `owner/repo` in the GitHub Actions ecosystem with `ref` as its version.

## Rationale

An action is a repository: a reference to a sub-directory still runs whatever
that repository holds at the given ref, so the repository is the dependency.

## Acceptance criteria

1. `uses: actions/checkout@v4` resolves to package `actions/checkout`, version
   `v4`.
2. A step of `.github/actions/setup/action.yml` using `actions/cache@<commit>`
   is recorded as a dependency of that action file.
3. A reference with fewer than two path segments is reported unresolved.
