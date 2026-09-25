---
id: REQ-JS-006
uuid: 5b53ed1c-2578-47e5-84c0-abdd1d3a590f
title: npm package resolution
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

The plugin **shall** resolve any other bare specifier to an npm package named by
its first path segment (its first two for a scoped `@scope/name`), with the
range declared in the nearest enclosing `package.json` that lists it under
`dependencies`, `devDependencies`, `peerDependencies` or `optionalDependencies`,
and **shall** mark a package no such `package.json` declares as unresolved.

## Rationale

A sub-path import belongs to its package; the nearest manifest is the one npm
installs from.

## Acceptance criteria

1. `@scope/tool/sub` resolves to the npm package `@scope/tool`.
2. `react-dom/client` with `react-dom` undeclared resolves to `react-dom`,
   unresolved.
3. `chalk` declared as `^5.3.0` and absent from any lock file has version
   `^5.3.0`.

## Notes

Specifiers beginning with `~`, `#` or `@/` (bundler aliases and package
`imports`) and URLs are dropped rather than treated as npm packages.
