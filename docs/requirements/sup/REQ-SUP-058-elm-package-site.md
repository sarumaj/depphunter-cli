---
id: REQ-SUP-058
title: The Elm package site for package dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a package of the
`elm` island depends on from the package site's files:
`<site>/packages/<author>/<name>/<version>/elm.json` at the version the
target names, else the newest version `releases.json` lists that the
target's range admits (the newest of all without a range). Its
`dependencies` are returned as the ranges they are, floating;
`test-dependencies` are left out. package.elm-lang.org is the public index;
a name that is not `author/name` is not asked.

## Rationale

The package site serves each release's `elm.json` as a file; there is no
other registry for Elm.

## Acceptance criteria

1. elm/html 1.0.0 yields elm/core, elm/json and elm/virtual-dom as ranges;
   `1.0.0 <= v < 2.0.0` asks `releases.json` and then 1.0.1, not 2.0.0.

## Notes

The sandbox proxy blocks package.elm-lang.org, so this was verified with a
stub server only.
