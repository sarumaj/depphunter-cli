---
id: REQ-ZIG-011
uuid: c9fd7e26-1778-4434-9f0f-db4b7db5aad1
title: Zig read without running Zig
scope: zig
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Zig **shall** be read without running the compiler or the build: imports are
literal, so none is missed, but build code is followed only through the
values listed in REQ-ZIG-006. Import names made in loops (`inline for` over
a tuple), by helper functions of other files or packages (a `translate_c`
helper's `addImportToModule`), or wired to generated modules are dropped, as
are compile steps whose roots are computed. `.paths` is read and not linked.
There is no Zig registry: `--online` looks nothing up, OSV has no Zig
ecosystem and Trivy no Zig package type, so no advisories are asked for.

## Rationale

The map is built from files alone.

## Acceptance criteria

1. In ghostty, `@import("glib")` wired in an `inline for` over the
   gobject modules is dropped.
