---
id: REQ-OCAML-009
uuid: 3ad6aa92-f562-43bb-a19f-7a360635bd7e
title: Locks and transitive dependencies
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `*.opam.locked` and `opam.locked` (opam's lock:
every package at `{= version}`) and dune package management's lock
directory `dune.lock/` beside a dune-project (from disk; one
`<package>.pkg` per package with its `version` and `depends`). The lock
directory **shall** answer `--resolve-depth`: a locked package's
dependencies at their locked versions, without the compiler and the
repository's own packages. opam's locks are flat and answer nothing, so
beyond them dependencies come from `--online` (REQ-SUP-054).

## Rationale

`opam lock` and `dune pkg lock` are how OCaml projects fix their
dependencies; only dune's lock records which package needs which.

## Acceptance criteria

1. `locked/src/app.ml`'s Lwt is lwt 5.9.1 from `locked/dune.lock`.
2. lwt 5.9.1's dependencies are cppo 1.8.0, dune 3.17.2 and ocplib-endian
   1.2, all pinned, without ocaml; lwt 5.7.0 and fmt 0.9.0 (opam's lock)
   answer nothing.
