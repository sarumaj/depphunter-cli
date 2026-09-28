---
id: REQ-COMMONLISP-012
title: Installed systems' dependencies
scope: commonlisp
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read the `.asd` files Qlot installed into `.qlot/dists/`
beside a `qlfile` or `qlfile.lock`, and ocicl into `systems/` beside an
`ocicl.csv`, from disk. `--resolve-depth` **shall** follow, for a Quicklisp
project found there by its primary system (named like the project, like a
git source's repository, or a system the project table maps to it), the
projects of the systems that system's `:depends-on` and
`:defsystem-depends-on` name (not `:weakly-depends-on`, `(:require ...)`,
the built-ins or its own secondary systems), each pinned as the pins of
the directory it was installed for say, and report them as installed. A
missing tree or an `.asd` that is not Lisp gives no dependencies.

## Rationale

`qlfile.lock` and `ocicl.csv` are flat lists; the installed systems' own
`.asd` files are the only offline record of what they depend on.

## Acceptance criteria

1. Qlot's dexador depends on cffi (by cffi-grovel), fast-http, quri and
   flexi-streams at the lock's dist version, not on cl+ssl, sb-bsd-sockets,
   UIOP or dexador/util; the git source github.com/fukamachi/dexador and
   cl-str (system str) are found too.
2. ocicl's dexador depends on quri pinned by `ocicl.csv` and on a floating
   fast-http.
