---
id: REQ-DLANG-006
title: Selections and pinning
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A dub package **shall** be named by its base name (`vibe-d` for
`vibe-d:http`, as the registry publishes it). `dub.selections.json` of the
package's root **shall** pin it (a version string; a `repository` entry by
its commit, with the repository as its origin), with the recipe's
specification as the requested version when it differs, and **shall** be
an import of it per entry; a `~branch` selection floats. Without a
selection, `==1.2.3` and a bare `1.2.3` (which dub reads as `==1.2.3`) pin,
`~>`, `^`, `>=`, ranges, `*`, `~branch` and no version float, and a
`repository` dependency pins by a commit only. A `path` dependency or
selection **shall** be an edge to its directory; the package's own
sub-packages (`:cli`, `shop:cli`) are edges to their directory, or dropped
when written inline; a sub-package depending on its package is an edge to
the package's directory.

## Rationale

dub.selections.json is dub's lock file; it is flat, so it pins versions
but records no graph.

## Acceptance criteria

1. `vibe-d` is pinned at 0.9.7 requested as `~>0.9.5`, `fancy` pinned at
   its commit with origin `https://github.com/acme/fancy.git`,
   `unit-threaded` (`"2.1.0"`) pinned, `eventcore` (`~master`
   selected) and `arsd-official:dom` (`~>11.0`) floating.
2. `widgets` is an edge to `libs/widgets`, `:cli` is dropped, `tools/` is an
   edge to `tools`.
