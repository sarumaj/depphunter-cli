---
id: REQ-SUP-060
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
is not asked. The registries dub asks before it **shall** be this machine's
(additive, trusted), in dub's order: `DUB_REGISTRY` (`;`-separated), then
the `registryUrls` of dub's settings files, the user's before the system's
(REQ-SUP-064); a repository's `dub.settings.json` names registries of its
own, untrusted. Only `http(s)` registries are recorded (dub's `file://` and
`mvn+` suppliers have no API). `skipRegistry` is honoured from the file of
highest priority that sets it: `standard` switches code.dlang.org off,
`configured` the settings' registries too, `all` every registry.

## Rationale

dub.selections.json is flat; the registry serves every version's recipe.

## Acceptance criteria

1. vibe-d 0.9.7 asks `/api/packages/vibe-d/0.9.7/info` only and yields
   diet-ng, eventcore and vibe-http; `~>0.9.5` asks `/api/packages/vibe-d/info`
   and reads 0.9.8, not 0.10.0, a pre-release or `~master`.
2. `DUB_REGISTRY`, then the user's and the system's `registryUrls` come
   before code.dlang.org; each `skipRegistry` value drops what dub drops; a
   repository's `dub.settings.json` registry is untrusted; a registry of the
   user's settings answers without code.dlang.org being asked.

## Notes

The sandbox proxy blocks code.dlang.org, so this was verified with a stub
server only.
