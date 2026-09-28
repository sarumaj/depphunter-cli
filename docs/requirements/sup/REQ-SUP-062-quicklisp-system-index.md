---
id: REQ-SUP-062
title: A Quicklisp dist's system index for project dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a project of the
`quicklisp` island depends on from its dist's system index: the distinfo
(the dist URL, `https://beta.quicklisp.org/dist/quicklisp.txt`, or for a
dated version `<dist>/<version>/distinfo.txt` beside it) names
`system-index-url`, a text file of lines `project system-file system-name
dependency ...` read once per dist version. The dependencies of the
project's own systems (the one named like the project, else each `.asd`
file's primary system, test systems left out) **shall** be returned, each
named by the project releasing it, without ASDF, UIOP and SBCL's contribs;
a dated version answers at that same dist version (pinned). A name that is
not a Quicklisp project name (a git source's repository name) is not
asked. A dist without the project **shall** say so, so that the next dist is
asked; a dist whose distinfo or system index cannot be read **shall not** be
asked again for another project until the retry interval (REQ-SUP-032) has
passed. The dists a qlfile adds (`dist <url>`, `dist <name> <url>`) are asked
before the Quicklisp dist and an `ultralisp <project>` line names the
Ultralisp dist (`https://dist.ultralisp.org/`) as that project's, all of
them the repository's (untrusted); the `distinfo-subscription-url` of each
dist installed in `~/quicklisp/dists` is this machine's (asked before the
Quicklisp dist, trusted). The Quicklisp dist itself is never recorded.

## Rationale

ASDF systems name their dependencies only in `.asd` files, which a dist
collects into its system index; one request answers every project.

## Acceptance criteria

1. dexador yields alexandria, babel, bordeaux-threads, cl-ppcre and
   usocket from the current dist; babel at `2023-10-21` asks that
   version's distinfo and yields its dependencies at that version; each
   system index is fetched once.
2. A project the machine's corp dist does not list is answered by the
   Quicklisp dist; a failing distinfo is asked once for three projects and
   again once the failure is older than the retry interval.
3. A qlfile's `dist` lines and `ultralisp` line and `~/quicklisp`'s installed
   dists are recorded as stated, the Quicklisp dist's own lines not.
