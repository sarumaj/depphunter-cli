---
id: REQ-JS-007
title: package-lock.json and npm-shrinkwrap.json versions
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** replace a declared range by the version the nearest
enclosing `package-lock.json` (formats v1 to v3) records for the package's
top-level installation, keeping the range as the requested specifier, and
**shall** treat that version as pinned. `npm-shrinkwrap.json` **shall** be
read the same way and, as npm does, in place of a `package-lock.json` in the
same directory. The `peerDependencies` of a v2 or v3 `packages` entry, which
npm 7 onwards installs, **shall** be dependency edges (REQ-SUP-009), each
resolved by the same `node_modules` walk as the package's dependencies; one
that `peerDependenciesMeta` marks optional **shall** be an edge only where the
lock installs it.
A git dependency **shall** be pinned to the commit its `resolved` (v2, v3) or
`version` (v1) names, from its repository (REQ-JS-019).

## Rationale

The manifest states what was asked for, the lock file what is installed; both
are shown.

## Acceptance criteria

1. `react` declared `^18.2.0` and locked at `18.3.1` resolves to version
   `18.3.1`, requested `^18.2.0`, pinned.
2. Beside a `package-lock.json` locking react at `18.2.0` and chalk, an
   `npm-shrinkwrap.json` locking react at `18.3.1` gives `18.3.1`, chalk
   keeps its range, and the shrinkwrap's edges are what `--resolve-depth`
   walks; another directory's `package-lock.json` still answers there.
3. In the npm v3 lock tree fixture, `e`'s peer `d` is an edge to the hoisted
   `d@1.0.0`, `b`'s optional peer `e` (installed) is an edge, and `e`'s
   optional peer `not-installed` adds nothing.
