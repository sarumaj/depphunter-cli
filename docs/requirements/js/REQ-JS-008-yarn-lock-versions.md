---
id: REQ-JS-008
title: yarn.lock versions
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read versions from `yarn.lock` files in both the classic
and the Berry format, **shall** select the entry whose descriptor matches the
range declared in `package.json` (also as `npm:<range>`), and, where the range
is written differently from every descriptor, **shall** accept the locked
version only when the lock holds a single version of the package. The lock
**shall** apply to the `package.json` files in its directory and below.
Berry entries **shall** be read with their `resolution` (the real package of
an alias such as `c2@npm:c@^2.0.0`, and whether it is a `workspace:`,
`portal:`, `link:` or `file:` package of the project) and their
`dependencies`, which, like a classic entry's, give the dependency edges of
REQ-SUP-009, and their `conditions`, which mark the package's platforms
(REQ-JS-018); `peerDependencies`, `dependenciesMeta`, `bin` and the
`__metadata` entry **shall not** be read as dependencies or packages.

## Rationale

A yarn.lock can hold several versions of one package; only the declared range
tells which one the project uses.

## Acceptance criteria

1. In a classic lock holding `left-pad` at two versions, the range `1.x` selects
   `1.3.0`.
2. In a Berry lock, `react` declared `^18.2.0` resolves to `18.2.0`, pinned.
3. A dependency named `version-guard` inside an entry is not read as the entry's
   version.
4. In a Berry lock, `b`'s dependency `d: ^1.0.0` resolves to the entry keyed
   `d@npm:^1.0.0`, a `patch:` dependency whose entry is keyed with a locator
   to the entry of the range it patches, and `h-alias: "npm:h@^1.0.0"` to the
   package `h`; a `workspace:` dependency is an edge to the workspace's
   directory (REQ-JS-004) and a `portal:` one outside the project adds no
   edge.
