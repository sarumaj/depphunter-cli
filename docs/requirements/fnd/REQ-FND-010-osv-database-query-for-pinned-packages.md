---
id: REQ-FND-010
uuid: 21c9421d-0c2d-4613-81a3-a0308bc04e8a
title: OSV database query for pinned packages
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the system **shall** ask the OSV database about every external
package the map pins to a version in the ecosystems Go, npm, PyPI, crates.io,
Maven, NuGet, GitHub Actions, Conan (as `ConanCenter`), Composer (as
`Packagist`, a version's leading `v` dropped), RubyGems, Swift packages (as
`SwiftURL`, by the URL they are named after), pub (as `Pub`), Hex (as `Hex`),
CRAN (as `CRAN`), Bioconductor (as `Bioconductor`), Hackage (as
`Hackage`), opam (as `opam`) and Julia (as `Julia`), and place each
matched advisory on its package with its fixed version.

## Rationale

Whether a pinned version is known to be vulnerable is the one question the
repository's own files cannot answer.

## Acceptance criteria

1. A pinned package with a known advisory yields a vulnerability finding on that
   package.
2. A package of an ecosystem OSV does not cover (PowerShell Gallery, container
   images, vcpkg, CocoaPods, Carthage, LuaRocks, Wally, CPAN) is not asked
   about.

## Notes

Packages declared private (scope `sup`) are not asked about either.
