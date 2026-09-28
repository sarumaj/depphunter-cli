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

`elm-tooling.json` in the project's directory or the nearest directory above
it **shall** be read for the compiler it pins (`tools.elm`, an exact
version): for a package, whose `elm-version` is a range, that version's
`ELM_HOME` directory is searched first for its installed dependencies
(REQ-ELM-007). The other tools it pins (elm-format, elm-json, elm-test-rs)
are programs run on the code, downloaded from their releases like the
compiler, and **shall not** be dependencies, as the `nim` requirement of a
`.nimble` file and the `crystal` field of `shard.yml` are not. A garbage or
missing file, or one pinning no exact version, changes nothing.

## Rationale

An application lists everything the build installs, indirect packages
included, so its `elm.json` puts the whole installed set on the map.

## Acceptance criteria

1. The fixture's `elm.json` imports its eight direct, two indirect and two
   test packages and the directories `src` and `lib/shared`;
   `../../outside` is dropped.
2. With `elm-tooling.json` pinning elm `0.19.0`, a package's `elm/json`
   range takes the version installed for 0.19.0 over the one installed for
   0.19.1; a garbage file, a range and a list leave 0.19.1's.
