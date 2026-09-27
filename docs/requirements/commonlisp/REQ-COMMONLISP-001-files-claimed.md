---
id: REQ-COMMONLISP-001
title: Files claimed
scope: commonlisp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Common Lisp plugin **shall** claim Lisp sources (`.lisp`, `.lsp`, and
`.cl` when the scan labels it Common Lisp), ASDF system definitions
(`.asd`), Qlot's `qlfile` and `qlfile.lock`, and ocicl's `ocicl.csv` (the
manifests told apart from other files by their names). A `.cl` file whose
head has a preprocessor line (`#include`, `#define`, `#pragma` ...) or an
OpenCL kernel declaration (`__kernel`, `kernel void`, `__global`) **shall**
be labelled OpenCL and not be claimed. Nothing in a `.qlot/` directory
(what Qlot installed) or in a `systems/` directory beside an `ocicl.csv`
(what ocicl downloaded) **shall** be claimed, and a walk of the file system
**shall** not enter them.

## Rationale

`.cl` is also the extension of OpenCL C kernels, which the Lisp reader
cannot read. Qlot and ocicl install the dependencies' sources into the
project; they are not the project's code.

## Acceptance criteria

1. The fixture's sources, `.asd` files, `qlfile` files, `qlfile.lock`,
   `ocicl.csv`, `lisp/old.cl` and `lisp/autolisp.lsp` are claimed;
   `opencl/kernel.cl` and `ocicl-app/systems/...` are not.
2. A binary `.lisp`, a `.cl` labelled OpenCL and paths under `.qlot/` are
   not claimed; `systems/x.lisp` without an `ocicl.csv` beside it is.
