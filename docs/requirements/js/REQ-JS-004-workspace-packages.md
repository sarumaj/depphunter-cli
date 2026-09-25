---
id: REQ-JS-004
uuid: c4b3af13-ad9f-4ab3-b307-4f7e12222539
title: Workspace packages
scope: js
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M2
verification:
  - unit
---

## Statement

The plugin **shall** resolve a specifier naming a package whose `package.json`
is part of the project to that package's directory, or, for a sub-path, to the
file the sub-path resolves to within it.

## Rationale

In a monorepo, workspace packages are local code and belong on the mainland, not
on the npm island.

## Acceptance criteria

1. `@acme/shared`, the name of `packages/shared/package.json`, resolves to
   `packages/shared`.
