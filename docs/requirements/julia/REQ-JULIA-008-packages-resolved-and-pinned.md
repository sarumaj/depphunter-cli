---
id: REQ-JULIA-008
title: Packages resolved and pinned
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A declared dependency **shall** resolve, in order, to a `[sources]` path
(a local package) or URL (Origin; a commit `rev` pins, else it floats), the
manifest's entry (pinned to its `version`, else its tree hash; `repo-url`
as Origin; the `[compat]` entry as Requested when it differs), a local
package with the declared UUID, the standard library, else a Julia package
as `[compat]` asks: `=1.2.3` pins; a bare `1.2` (a caret range in Pkg),
`~`, inequalities, hyphen ranges and lists are kept as written and float;
no entry floats. An absolute module no environment declares **shall** be an
unresolved Julia package named by its first segment. Julia packages
**shall** be asked about in OSV's `Julia` ecosystem when pinned.

## Rationale

Julia's `[compat]` reads a bare version as a caret range, unlike most
ecosystems; only an equality with all three parts names one release.

## Acceptance criteria

1. `=1.2.3` pins; `1.2.3`, `=1.2`, `~1.2, 2`, `1.2 - 1.5` and `>= 1`
   float with the entry as their version; no entry floats.
2. JSON resolves to 0.21.4 requested as `0.21`; HTTP to 1.10.8 without a
   Requested (`=1.10.8` is the same); `using Missing1` is unresolved.
3. OSV is asked about HTTP 1.10.8 as `Julia`, not about a `julia-std`
   package.
