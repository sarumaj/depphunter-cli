---
id: REQ-ADA-001
uuid: edf512ac-41af-4160-915b-51c5db542829
title: Files claimed
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Ada plugin **shall** claim Ada sources (`.ads` specs, `.adb` bodies
and `.ada` files of other naming schemes, in either case, SPARK
included), GNAT project files (`.gpr`) and Alire's crate manifest
(`alire.toml`, told apart from other TOML files by its name). Nothing in an
`alire/` directory beside an `alire.toml` (Alire's lock file, build files
and the crates it fetched) nor in an `obj/` directory beside a `.gpr`
(what a build wrote, such as the binder's `b__main.adb`) **shall** be
claimed; an `alire/` or `obj/` directory elsewhere is source.

## Rationale

Ada's units live in specs and bodies, a GNAT build is described by its
project files and an Alire crate by `alire.toml`; git lists what a
repository commits in `alire/` or `obj/` even though a walk skips them.

## Acceptance criteria

1. The fixture's sources, `shop.gpr`, `libs/widgets/widgets.gpr` and both
   `alire.toml` files are claimed; `alire/alire.lock`, the crates under
   `alire/cache/dependencies/` and `obj/b__shop-main.adb` are not.
2. `docs/alire/intro.ads` and `obj/x.adb` in a repository without a
   manifest or project file there are claimed; a binary `.ads` is not.
