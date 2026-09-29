---
id: REQ-JS-004
title: Workspace packages
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a specifier naming a package whose `package.json`
is part of the project to that package's directory, or, for a sub-path, to the
file the sub-path resolves to within it. Where a lock file's npm package
depends on a workspace package (a `workspace:` range, npm's link to a
workspace, pnpm's `link:`, or a name no lock file installs from a registry and
a `package.json` of the project has), the `depends` edge **shall** go to that
package's directory, not to an npm package of the name.

## Rationale

In a monorepo, workspace packages are local code and belong on the mainland, not
on the npm island.

## Acceptance criteria

1. `@acme/shared`, the name of `packages/shared/package.json`, resolves to
   `packages/shared`.
2. In the lock tree fixtures (npm v1 and v3, Berry, pnpm v9), `a`'s dependency
   on the workspace `ws-lib` is a `depends` edge to the directory
   `packages/ws`, and no npm node `ws-lib` exists.
3. In `bun.lock`, `@scope/dep`'s dependency on `ui`, which the lock installs
   only as the workspace, is an edge to `packages/ui`.
