---
id: REQ-CPP-010
title: Conan manifests read
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** read the references of every `conanfile.txt`
(`[requires]`, `[tool_requires]`, `[build_requires]`,
`[test_requires]`) and the string literals of every `conanfile.py` given to
`self.requires()`, `self.tool_requires()`, `self.build_requires()` and
`self.test_requires()` or assigned to the `requires`, `tool_requires`,
`build_requires` and `test_requires` attributes (a string, several, a list
or a tuple). A reference `name/version@user/channel#revision` **shall** name
the Conan package `name`, pinned when its version is exact and floating when
it is a range in brackets (`[>=1.0 <2]`, `[~1.2]`).

## Rationale

A Conan reference names one version unless it is written as a range.

## Acceptance criteria

1. `zlib/1.2.13` in `[requires]` is Conan `zlib` 1.2.13, pinned.
2. `requires = "openssl/1.1.1t", "zlib/[~1.2]"` gives `openssl` pinned
   and `zlib` floating at `[~1.2]`.
3. A commented-out requirement and an f-string are not read.

## Notes

See REQ-CPP-013 for what reading `conanfile.py` as text misses.
