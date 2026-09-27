---
id: REQ-SUP-054
title: opam package dependencies from opam-repository
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read an opam package's dependencies from an
opam repository laid out as files
(`https://raw.githubusercontent.com/ocaml/opam-repository/master` unless
configured otherwise): `<repository>/packages/<name>/<name>.<version>/opam`,
for a package with an exact version; a package with a range or none
**shall** not be asked about (no version to ask for). The answer is the
description's `depends` without the compiler's packages and what only
tests, documentation or development need (`with-test`, `with-doc`,
`with-dev-setup`, `dev`); a dependency written `{= "1.2"}` is that version
(pinned), else its constraint as written.

## Rationale

opam.ocaml.org serves the repository as an archive and its package pages
as HTML; the git repository serves each description as a file, which is
one request per package and version.

## Acceptance criteria

1. Against a stub server, lwt 5.9.1 gives cppo `>= 1.1.0`, dune `>= 3.8`
   and ocplib-endian 1.2 (pinned); ocaml, base-threads, with-test and
   with-doc dependencies and a repeated package are left out.
2. fmt `>= 0.9` is not asked about; with nothing configured, the index for
   `opam` is the opam-repository files URL, known.

## Notes

Machine and project opam repository configuration (`opam repo`,
dune-workspace `repository` stanzas) is not discovered.
