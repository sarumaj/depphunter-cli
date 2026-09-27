---
id: REQ-ELM-006
uuid: 35956870-e9d5-435b-92a4-38dbb0b7d48c
title: Pinning
scope: elm
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package an application's `elm.json` lists **shall** be pinned at its exact
version; a package a package's `elm.json` lists **shall** float, with its
range (`1.0.0 <= v < 2.0.0`) as the version. An import resolved to a
package takes the version the importing file's project lists; a package the
project does not list is unresolved.

## Rationale

Elm applications record exact versions of every package; packages give
ranges, and the application using them decides.

## Acceptance criteria

1. `Html` in `src/Main.elm` is elm/html 1.0.0, pinned; in
   `packages/ui-kit` it is elm/html `1.0.0 <= v < 2.0.0`, floating.
2. `Browser` in an application not listing elm/browser is the unresolved
   package elm/browser.
