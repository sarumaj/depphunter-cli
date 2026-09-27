---
id: REQ-CPP-009
title: vcpkg manifests read
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** read every `vcpkg.json`: its dependencies and
those of its features, each a port name or an object with `name` and
`version>=` (`features` and `platform` are accepted and not evaluated), and
its `overrides`. A port
with an override **shall** be pinned to the override's version, with the
minimum it replaced as the requested version; a port with only a minimum
**shall** carry `>=` and the minimum and float; a port without either
**shall** be marked floating unless a `builtin-baseline` or a baseline of the
registry serving it (`vcpkg-configuration.json` or the manifest's
`vcpkg-configuration`) fixes its version.

## Rationale

An override is the only place a vcpkg manifest names one exact version. A
baseline fixes versions too, but through the registry's version database,
which the repository does not carry: the map can name no version for it, so it
neither pins (a pinned package names its version) nor warns.

## Acceptance criteria

1. `{ "name": "openssl", "version>=": "3.0.8" }` with an override to 3.2.0
   is vcpkg `openssl` 3.2.0, pinned, requested as `>=3.0.8`.
2. `{ "name": "boost-asio", "version>=": "1.83.0" }` is `>=1.83.0`, not
   pinned.
3. A dependency listed only under a feature (`opencv4`) counts as declared.
4. `"catch2"` in a manifest with no baseline is floating; with a
   `builtin-baseline`, or served by a registry with a baseline, it is not.

## Notes

The versions a baseline selects and what a port depends on are not read, so
vcpkg packages are not resolved beyond the first level.
