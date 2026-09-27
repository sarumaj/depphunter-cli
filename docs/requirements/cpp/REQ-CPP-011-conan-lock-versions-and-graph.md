---
id: REQ-CPP-011
title: conan.lock versions and graph
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** read a `conan.lock` beside a conanfile, in the
Conan 2 form (`requires`, `build_requires` lists) and the Conan 1 form
(`graph_lock` nodes): a declared package the lock holds **shall** be pinned
to the locked version, with a different declared version as the requested
one, and a package only the lock holds **shall** count as declared, pinned.
The edges of a Conan 1 lock **shall** answer what a Conan package depends on
for `--resolve-depth`. With `--online`, pinned Conan packages **shall** be
asked about in OSV's `ConanCenter` ecosystem.

## Rationale

A lock resolves the ranges and holds every library the build installs,
including those only another library needs, whose headers the project may
include as well.

## Acceptance criteria

1. `spdlog/[>=1.11 <2]` locked at 1.12.0 is 1.12.0, pinned, requested as
   `[>=1.11 <2]`.
2. `fmt` that only the lock holds resolves `<fmt/format.h>` to Conan
   `fmt` 10.1.1.
3. In a Conan 1 lock where `libcurl/8.4.0` requires `openssl/1.1.1t` and
   `zlib/1.2.13`, those are its dependencies.

## Notes

A Conan 2 lock is a flat list without edges, so its packages are not resolved
beyond the first level. vcpkg has no OSV ecosystem.
