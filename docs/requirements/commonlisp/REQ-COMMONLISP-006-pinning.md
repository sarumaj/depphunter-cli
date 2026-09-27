---
id: REQ-COMMONLISP-006
title: Qlot and ocicl manifests and pinning
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The entries of a `qlfile` (`ql`, `ultralisp`, `ql-dist`, `git`, `github`,
`http`, `local`), of a `qlfile.lock` and of an `ocicl.csv` **shall** be
imports, and they **shall** pin the systems of files below their
directory (the nearest directory with any; none above a file, all of the
repository's): an `ocicl.csv` row pins its project by the image digest,
showing the release's version; a lock pins a Quicklisp project to its dist
version and a git one to its commit, with the `qlfile`'s version, branch
or tag as requested; without a lock, a dist version (`ql x 2023-10-21`)
and a git commit (`:ref`) pin, a tag is shown (neither), and `:latest`, a
branch or nothing float; an `http` tarball with its md5 pins. A project
nothing lists **shall** be pinned by the lock's dist version (the last
dist, which has priority), else by `ql :all <version>` or the Quicklisp
dist's version in a `dist` line, else float. Git sources **shall** be
named by their repository (`github.com/fukamachi/dexador`), with the URL
as origin off the public forges; a `local` directory in the repository is
an edge to it.

## Rationale

Quicklisp dist versions are immutable snapshots of every project, so a
dist version pins as a commit does; ASDF alone names no versions.

## Acceptance criteria

1. The fixture's lock pins alexandria (requested `latest`), cl-ppcre,
   dexador and str by commit (requested `master`, `0.21`) and every other
   Quicklisp project at the dist's `2023-10-21`.
2. `tools/qlfile` (no lock) pins by `ql :all`, a `:ref`, an md5; shows a
   tag; floats `:latest`, a branch and ultralisp; `ocicl-app` pins by its
   `ocicl.csv` and floats `(:version "fiveam" "1.4")` as `>= 1.4`.
