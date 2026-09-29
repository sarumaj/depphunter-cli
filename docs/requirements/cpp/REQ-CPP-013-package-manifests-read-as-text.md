---
id: REQ-CPP-013
title: Package manifests read as text
scope: cpp
type: limitation
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The C/C++ plugin **shall not** run a `conanfile.py` or run vcpkg or Conan:
references built at run time (f-strings that substitute, variables, a
`requirements()` computing them) are not seen, a conditional requirement
counts whatever its condition, a vcpkg `platform` or feature is not
evaluated, the versions a vcpkg baseline selects are not known, and headers are
matched to packages by name, not by the files a package installs. With
`--online`, a Conan remote's recipes are read the same way, as text
(REQ-SUP-076); vcpkg's registries are not asked.

## Rationale

Running a recipe runs arbitrary code, and running the package managers needs
them installed and, for vcpkg's baseline, the registry's history.

## Acceptance criteria

1. Inspection of internal/lang/cpp/packages.go shows conanfile.py read with
   regular expressions over its text, and no process started.
