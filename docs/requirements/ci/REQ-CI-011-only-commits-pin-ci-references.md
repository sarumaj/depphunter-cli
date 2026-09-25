---
id: REQ-CI-011
uuid: 8af93d73-bc77-48af-a6ea-e38f11e24e04
title: Only a commit pins a CI reference
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** mark an action, a reusable workflow, a GitLab project
include or a GitLab component as pinned only when its ref is a full git commit
(40 or 64 hexadecimal characters), and **shall** treat any tag or branch as
floating.

## Rationale

A tag can be moved to other code at any time; only a commit is immutable.
Pinning is therefore stricter for CI references than for packages.

## Acceptance criteria

1. `actions/checkout@v4` is floating;
   `actions/setup-go@3041bf56c941b39c61721a86cd11f3bb1338122a` is pinned.
2. A workflow whose actions are all pinned to tags shows every action floating,
   and replacing the tags by commits makes them pinned.
3. A component at `@1.4.0` is not pinned.
4. A short commit such as `95032a82` does not pin.
