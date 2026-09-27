---
id: REQ-SUP-060
uuid: a1901392-25a9-497b-95d1-f32aa4197763
title: The dub registry for package dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a package of the
`dub` island depends on from a dub registry's API:
`<registry>/api/packages/<name>/<version>/info` for an exact version, else
`<registry>/api/packages/<name>/info` and the newest release (no
pre-release, no branch) the target's specification admits (`~>1.2.3` is
`>=1.2.3 <1.3.0`, `~>1.2` `>=1.2.0 <2.0.0`, `^`, `==`, a bare version,
comparisons, `*`). The dependencies of the version's recipe, of its
sub-packages and of its first (default) configuration are returned under
their base package's name as the specifications they are; optional and
path dependencies and the package's own sub-packages are left out.
code.dlang.org is the public index; a name that is not a dub package name
is not asked.

## Rationale

dub.selections.json is flat; the registry serves every version's recipe.

## Acceptance criteria

1. vibe-d 0.9.7 asks `/api/packages/vibe-d/0.9.7/info` only and yields
   diet-ng, eventcore and vibe-http; `~>0.9.5` asks `/api/packages/vibe-d/info`
   and reads 0.9.8, not 0.10.0, a pre-release or `~master`.

## Notes

The sandbox proxy blocks code.dlang.org, so this was verified with a stub
server only.
