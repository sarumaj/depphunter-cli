---
id: REQ-ADA-005
uuid: 404610d9-c74d-4d95-93ad-41ae8ba22de7
title: GNAT project files
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A project file's `with` (and `limited with`), `extends` and aggregate
`Project_Files` **shall** resolve to the project file of the repository
they name (relative to the file, else the nearest of that name), else to
the Alire crate that ships it (a crate Alire fetched with that project
file, a declared crate of that name or a curated table's), else to an
unresolved crate; a missing project named by a path or after the
repository's own crate (`config/shop_config.gpr`) **shall** be dropped.
`Source_Dirs` entries (`dir/**` included) **shall** be edges to the
directories and `Main` entries to the main files. The file **shall** be
read without being evaluated: every alternative of a case construct
counts, a variable holds what the file assigns it, an `external` its
default, and what refers to other projects' attributes is unknown.

## Rationale

GPRbuild builds from the project files, whatever the crate manifest says.

## Acceptance criteria

1. `shop.gpr` links `aunit` to the fetched crate that ships
   `aunit.gpr`, `gnatcoll` and `xmlada` to the declared crates,
   `libs/widgets/widgets.gpr` to that file, `src`, `src/debug` (from
   `external ("SHOP_MODE", "debug")`) and `tools` to directories, and
   both case alternatives' mains to their files.
