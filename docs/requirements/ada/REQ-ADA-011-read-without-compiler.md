---
id: REQ-ADA-011
title: Read without the compiler or alr
scope: ada
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Ada **shall** be read without running GNAT, GPRbuild or alr: only the
first branch of a gnatprep `#if` is read, project files are not
evaluated (every case alternative counts, other projects' attributes and
`GPR_PROJECT_PATH` are unknown), a unit generated at build time (Alire's
`<crate>_config`, wsdl2aws output) is dropped or unresolved, a unit of a
crate Alire has not fetched is attributed by declared names and a curated
table, and with `--online` alr's solver is not run: a range stands for the
newest release it admits in an index alr keeps a checkout of or GitHub
lists, and only an exact version is asked of an index served over HTTP
otherwise (a file server cannot list a crate's releases;
[REQ-SUP-061](../sup/REQ-SUP-061-alire-community-index.md)).

## Rationale

Which branch, scenario and crate version a build uses is known only to the
build and the solver.

## Acceptance criteria

1. `with Real.C` in a gnatprep `#if` branch is read and `with Fake.E` in
   its `#else` branch is not.
