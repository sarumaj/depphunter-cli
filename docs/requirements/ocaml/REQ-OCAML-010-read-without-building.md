---
id: REQ-OCAML-010
title: OCaml read without building
scope: ocaml
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

OCaml **shall** be read without running dune, opam or the compiler: names
brought into scope by opening a submodule of a local module
(`open Test.Import`), by opening a package's module, by an `include` of a
package, or by ppx rewriters and preprocessors (cppo branches are read
together) are not followed and such references are dropped; a library's
modules are not read from installed packages, so a module of a package is
matched by names (the library's main module, the package's prefix, a curated
table); dune rules that generate modules are not run (a `x.cppo.ml` stands
for `x.ml`); `(subdir)` stanzas, `(vendored_dirs)` and virtual libraries'
implementations are not read. With `--online`, opam's solver is not run
either: an unpinned package stands for the newest version its constraint
admits in its repository, and a range is not asked of a repository served
over HTTP that this machine keeps no copy of and GitHub does not list
([REQ-SUP-054](../sup/REQ-SUP-054-opam-repository-files.md)).

## Rationale

The map is built from files alone; what the type checker knows about
module contents is not in them.

## Acceptance criteria

1. In a sample, `Real.Sig` and `Real.S` (module types) name the module
   `Real`; references a file cannot place are dropped rather than invented.
