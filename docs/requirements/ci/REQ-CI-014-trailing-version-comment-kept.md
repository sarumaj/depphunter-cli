---
id: REQ-CI-014
uuid: 48ad5fc2-e82f-4399-903f-b59390272c64
title: Version comment of a commit-pinned reference
scope: ci
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Where an action or reusable workflow is pinned to a commit and its line carries
a trailing comment, the CI plugin **shall** keep the first word of that comment
(such as `v4.1.1`) as the requested version.

## Rationale

Hardening guides require pinning actions to commits, which leaves the readable
version only in a trailing comment; keeping it shows which release the commit
stands for.

## Acceptance criteria

1. `actions/setup-go@3041bf5… # v5.0.1` resolves to the commit, requested
   `v5.0.1`, pinned.
2. `actions/cache@0c907a7… # v4.2.1` in a composite action resolves to requested
   `v4.2.1`.
3. A comment beside a tag-pinned reference sets no requested version.
