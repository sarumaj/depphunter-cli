---
id: REQ-SUP-009
title: Lock files read for the transitive walk
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The walker **shall** read package-to-package dependencies from
`package-lock.json` versions 1 to 3 (and `npm-shrinkwrap.json`, which npm
reads in its place), `pnpm-lock.yaml` versions 5 to 9 (edges under
`packages` up to v8, under `snapshots` in v9, peer context such as
`(react@18.2.0)` or v5's `_react@18.2.0` dropped), classic and Berry
`yarn.lock`, `bun.lock` (a package installed under another, `a/b`, being
what `a` requires), `Cargo.lock`, `uv.lock`, `poetry.lock`, `pdm.lock`,
`composer.lock` (or, without one, `vendor/composer/installed.json`),
`Gemfile.lock` and, for Swift packages, the manifests SwiftPM checked out
under `.build/checkouts` pinned by `Package.resolved`.

For the npm lock formats the version of each dependency **shall** be that of
the copy the requiring package loads: in `package-lock.json` the install path
Node finds walking up from the requiring package's own path
(`node_modules/a/node_modules/b` before `node_modules/b`, v1's nested
`dependencies` read as the same paths); in `yarn.lock` the entry whose
descriptors contain the dependency's `name@range` (also as `name@npm:range`,
and for a `patch:` range the range it patches); in `pnpm-lock.yaml` the
version the entry names; npm 7's peer dependencies are edges too
(REQ-JS-007). A package installed under an alias (`"c2":
"npm:c@^2"`) **shall** be reached as the real package, and one the project
imports under an alias **shall** have the real package's dependencies. A
workspace, `portal:`, `link:` or `file:` dependency **shall not** become a
package, and neither shall an optional dependency the install left out; one
naming a workspace package of the project **shall** be an edge to its
directory (REQ-JS-004). A package installed on some platforms only **shall**
stay, marked with them (REQ-JS-018). Where
one name is installed at several versions, its node **shall** keep the edges
of every copy.

## Rationale

Each of these files already records the whole resolved graph, each in its own
shape, so the walk needs nothing but the repository.

## Acceptance criteria

1. For each listed format, a fixture yields the dependencies the file records
   for a package, with the locked version and pinned.
2. A crate present in two versions in `Cargo.lock` is resolved to the version
   the lock names.
3. The same install written as `package-lock.json` v1 and v3, Berry
   `yarn.lock` and `pnpm-lock.yaml` v5, v6 and v9 gives, for a package whose
   dependency is installed twice (a nested copy and a hoisted one), the
   version of the copy that package loads, and the same `--resolve-depth -1`
   graph, with aliases, patches, workspaces, portals and platforms as stated.
