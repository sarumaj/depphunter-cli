---
id: REQ-COMMONLISP-008
title: Built-ins
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The standard's packages (`common-lisp`, `cl-user`, `keyword`), an
implementation's own (`sb-*` as sbcl, `ccl`, `excl`, `lispworks`,
Genera's `future-common-lisp`), ASDF and UIOP (every implementation ships
them, `uiop/*` and `asdf/*` included) and the Quicklisp client's (`ql`)
**shall** resolve to the hidden cl-std island, one package per
implementation or tool; so **shall** the systems `asdf`, `uiop` and SBCL's
contribs (`sb-posix`), and modules `require` loads from an implementation
(`gray-streams`). A `require` of a file (`"streamc.fasl"`) is dropped.

## Rationale

These are always present and have no version of the project's choosing.

## Acceptance criteria

1. `use cl`, `uiop:`, `sb-ext:`, `ql:`, `depends-on sb-posix`,
   `depends-on uiop` and `require gray-streams` resolve to cl-std.
