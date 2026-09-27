---
id: REQ-COMMONLISP-007
title: Systems and packages to Quicklisp projects
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An external system **shall** be the Quicklisp project releasing it:
a secondary system's (`ironclad/digests/sha256`) primary system, a
curated table of systems released by projects of other names
(`cl-ppcre-unicode` by cl-ppcre, `str` by cl-str, `cl+ssl` by
cl-plus-ssl, `swank` by slime, clack's handlers, lack's middlewares,
cl-dbi's drivers), else the system's name. A package **shall** be matched
to a system through a curated table of packages named otherwise (`bt2`,
`ppcre`, `dex`, `5am`, `c2mop`, `lack.request`).

## Rationale

Quicklisp distributes projects; a project releases one or more systems,
and a system defines packages whose names often differ.

## Acceptance criteria

1. The projects test maps the systems and packages above.
