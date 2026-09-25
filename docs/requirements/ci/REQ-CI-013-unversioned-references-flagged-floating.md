---
id: REQ-CI-013
uuid: 43df0dbe-ee9b-4a27-afa3-70d32ecd01e4
title: Unversioned CI references flagged floating
scope: ci
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The CI plugin **shall** mark a GitLab template include, a remote include, a
project include without `ref` and an action or reusable workflow reference
without `@ref` as floating through the target's floating flag, without inventing
a version for it.

## Rationale

Such a reference names no version at all yet moves with whatever its source
serves today; without a flag of its own it would read as "nothing known" rather
than as the loosest dependency there is.

## Acceptance criteria

1. `template: Security/SAST.gitlab-ci.yml` is floating with an empty version.
2. `remote: https://example.com/ci/shared.yml?v=2` is floating with an empty
   version.
3. A project include without `ref` is floating with an empty version.
4. A component reference without `@version` is floating.

## Notes

Partial: a GitLab component without `@version` resolves with no version and no
floating flag, so it is neither pinned nor floating (criterion 4). A container
image without a tag is given version `latest` instead of the flag (REQ-CI-012),
which is the reference Docker itself uses.
