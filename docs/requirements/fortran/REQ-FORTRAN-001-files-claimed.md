---
id: REQ-FORTRAN-001
uuid: ae58171e-53fd-4919-8d0d-801f24125e70
title: Files claimed
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Fortran plugin **shall** claim Fortran sources in free form (`.f90`,
`.f95`, `.f03`, `.f08`, `.f18` and their upper-case, preprocessed spellings
such as `.F90`) and fixed form (`.f`, `.for`, `.ftn`, `.f77`, `.fpp` and
upper-case `.F`, `.FOR`), fypp templates (`.fypp`) and fpm's `fpm.toml`,
telling `fpm.toml` apart from other TOML files by name. A `.f` or `.for`
file that scan labels Forth (REQ-LANG-015) **shall** not be claimed, nor
anything in a `build/` directory beside an `fpm.toml` (what fpm built and
the dependencies it fetched into `build/dependencies/`). Include files
(`.inc`, `.h`, `.fi`) **shall** not be claimed: `.inc` is PHP's, and an
include file is a fragment, linked from the file that includes it.

## Rationale

Fortran's extensions are many and old; `.f` is also Forth's, and fpm's
`build/` holds copies of other packages' sources that would read as the
project's own.

## Acceptance criteria

1. The fixture's sources (`src/cart.F90`, `legacy/dgemm.f`,
   `legacy/tabbed.for`) and both `fpm.toml` files are claimed;
   `legacy/words.f` (Forth), `build/cache.toml`,
   `build/dependencies/fancy/src/fancy_core.f90`, `src/shop.inc` and
   `src/config.h` are not.
2. A `.f` file starting with a `\` comment or `: name ... ;` is Forth and
   not claimed.
