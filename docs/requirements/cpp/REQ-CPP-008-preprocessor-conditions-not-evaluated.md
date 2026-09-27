---
id: REQ-CPP-008
uuid: 8c17b716-77c0-4490-b255-a51738fe4138
title: Preprocessor conditions not evaluated
scope: cpp
type: limitation
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The C/C++ plugin **shall not** evaluate preprocessor conditions other than
a literal 0 or 1, expand macros, or read build files (CMake, Meson, Make) or
package manifests; includes of every platform branch are recorded, and a C
header that uses C++ keywords as names may lose definitions.

## Rationale

Conditions depend on the compiler, the platform and the build's definitions,
which differ between builds of one project; an extraction depends only on the
file's content and extension.

## Acceptance criteria

1. Inspection of internal/lang/cpp shows no evaluation of `#ifdef` or `#if`
   conditions besides literal 0 and 1, and no reading of CMake files.
