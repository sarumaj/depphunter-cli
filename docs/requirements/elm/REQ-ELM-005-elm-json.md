---
id: REQ-ELM-005
title: elm.json dependencies and source directories
scope: elm
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Elm plugin **shall** read `elm.json`: an application's
`dependencies.direct`, `dependencies.indirect`,
`test-dependencies.direct` and `test-dependencies.indirect`, a package's
`dependencies` and `test-dependencies`. Each listed package **shall** be an
import of the `elm.json` on the line naming it, in the `elm` island as
`author/name`, and each source directory an edge to that directory when it
is in the repository. An `elm.json` without an `application` or
`package` type gives nothing.

## Rationale

An application lists everything the build installs, indirect packages
included, so its `elm.json` puts the whole installed set on the map.

## Acceptance criteria

1. The fixture's `elm.json` imports its eight direct, two indirect and two
   test packages and the directories `src` and `lib/shared`;
   `../../outside` is dropped.
