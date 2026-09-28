---
id: REQ-CUE-011
title: Dependencies from the module cache
scope: cue
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

For `--resolve-depth`, a module dependency at an exact version **shall**
depend on the `deps` of its own `cue.mod/module.cue` in cue's module cache
(`$CUE_CACHE_DIR`, else `cue` in the user's cache directory;
`mod/extract/<module>@<version>/`, the path escaped as Go escapes upper-case
letters), each at the version the repository's `module.cue` selects when
it lists the module, else at the version the dependency's file names, and
be reported as installed. A module the cache does not hold, one without an
exact version and a `module.cue` that is not CUE give no dependencies.
Nothing is fetched.

## Rationale

`cue` extracts every module it fetched into the cache; like Go's `go.mod`,
the main module's `module.cue` lists every module of the build at its
selected version.

## Acceptance criteria

1. github.com/Acme/lib v0.2.0, cached under `github.com/!acme/lib@v0.2.0`,
   depends on cue.dev/x/k8s.io at the repository's v0.5.0 (not its own
   v0.4.0) and github.com/other/util v0.1.0; util, whose cached file is
   garbage, and an unversioned module depend on nothing.
