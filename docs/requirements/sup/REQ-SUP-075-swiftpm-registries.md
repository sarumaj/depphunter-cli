---
id: REQ-SUP-075
title: SwiftPM package registries for registry packages
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The registries SwiftPM's `registries.json` names **shall** be read: the
user's (`~/Library/org.swift.swiftpm/configuration` on macOS when it has
one, else `configuration` below `$XDG_CONFIG_HOME/swiftpm` or `~/.swiftpm`)
as this machine's, and the one in `.swiftpm/configuration` beside each of the
repository's `Package.swift` files (read from the disk) as the repository's.
Each maps package scopes to registries, `[default]` serving every other
scope; scopes compare without case. A file SwiftPM would refuse (a format
version other than 1, a key that is no scope) **shall** name nothing.

A registry package (`.package(id: "scope.name")`, a `registry` pin) **shall**
be asked of one registry, as SwiftPM merges the files: the repository's
registry for its scope, else the user's, else the repository's default, else
the user's. A package named by its repository's URL is asked of none. A
registry only the repository names is not asked unless the user vouches for it
or the user's own `registries.json` names the same registry; a credential this
machine holds for its host does not vouch for it (REQ-SUP-043).

With `--online`, the index client **shall** answer as the Swift Package
Registry API (SE-0292) is asked: `GET <registry>/<scope>/<name>` lists the
releases (`application/vnd.swift.registry.v1+json`), and `GET
<registry>/<scope>/<name>/<version>/Package.swift`
(`application/vnd.swift.registry.v1+swift`) is one release's manifest. An
exact version **shall** be read as it is; otherwise the newest release
without a `problem` that is not a pre-release and that the requirement admits
(`a..<b`, `a...b`), else the newest such release. The manifest's registry
(`.package(id:)`) and repository (`.package(url:)`) dependencies **shall** be
the answer, read as the swift plugin reads manifests, local paths left out.

## Rationale

A registry is where SwiftPM gets a registry package's manifest; without it
`Package.resolved` gives the packages but not what they depend on.

## Acceptance criteria

1. With the user's default and `acme` registries and the repository's
   default and `mona` registries, `mona.*` is the repository's (untrusted),
   `acme.*` the user's, other scopes the repository's default; a URL-named
   package has no registry; the user vouching makes the repository's known,
   and a netrc credential for its host does not.
2. `1.0.0..<2.0.0` reads 1.1.0, skipping a withdrawn 1.2.0 and a
   pre-release, and answers `mona.Collections`, `acme.Exact` (pinned) and
   `github.com/apple/swift-nio`; an exact version skips the listing.
3. A registry only the repository names is not asked, and the package is
   reported as coming from it.

## Notes

The swift-package-manager sources were read for the file locations, the
format, the merge and the API paths (the registry hosts are not reachable
from the sandbox). Not read: an Xcode project's own `registries.json` (in its
workspace's `xcshareddata/swiftpm`), `replaceScmWithRegistry` and the
registry's `GET /identifiers?url=` lookup, which would turn a URL-named
package into a registry one, and the `SWIFTPM_REGISTRY_TOKEN` and
`SWIFTPM_NETRC_DATA` variables of newer toolchains.
