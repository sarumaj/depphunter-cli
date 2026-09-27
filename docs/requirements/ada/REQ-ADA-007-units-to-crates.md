---
id: REQ-ADA-007
title: Units attributed to crates
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A unit no file of the repository declares **shall** be a predefined unit
of the `ada-std` island when it is Standard, an Ada 83 renaming
(`Text_IO`, `Unchecked_Conversion`, ...) or under `Ada`, `System`,
`Interfaces` or `GNAT`, named by its first two segments
(`ada.containers`); else the crate Alire fetched that has the unit or its
nearest parent unit; else the declared, pinned, locked or fetched crate
the unit's name spells (segments joined by `_`, with an `ada_` prefix,
an `_ada` or `ada` suffix or a `lib` prefix: `Acme_Log.Sinks` is
acme_log) or a curated table names (`AWS`, `GNATCOLL`, `AUnit`,
`Templates_Parser`, XML/Ada's `Sax`/`DOM`/`Unicode`, GtkAda's `Gtk`,
`Libadalang`, `VSS`, `TOML`, ...), the longer match winning; else the
table's crate, unresolved; a unit of the repository's own crate or of a
first segment only the repository's units have **shall** be dropped (it is
generated at build time); else an unresolved crate named by the unit's
first segment in lower case.

## Rationale

Alire crates do not declare which units they provide; the fetched crates
do, and names spell the rest.

## Acceptance criteria

1. `AUnit.Assertions` and `Parsers.Generic_Source` resolve to the fetched
   aunit and simple_components, `DOM.Core` to xmlada, `TOML` to ada_toml,
   `Win32.Winbase` to win32ada, `Text_IO` to `ada-std` `text_io`,
   `Shop_Config` is dropped and `Mystery.Thing` is the unresolved crate
   `mystery`.
