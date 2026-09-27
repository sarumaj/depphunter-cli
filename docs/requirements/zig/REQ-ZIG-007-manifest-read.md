---
id: REQ-ZIG-007
uuid: 9fdbd49e-34e1-4ff7-8de7-ddbed39d2df6
title: build.zig.zon read
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `build.zig.zon` **shall** be read as a ZON literal: `.name` (an enum
literal since Zig 0.14, a string before), `.version`,
`.minimum_zig_version`, `.paths` and `.dependencies`, each with `.url` and
`.hash` or `.path`, and `.lazy`. Its name **shall** be its symbol and each
dependency an import whose spec is the dependency's name; a `.path`
dependency **shall** resolve to that directory (the repository's root
`build.zig.zon` for the root), and a lazy one like any other.

## Rationale

The manifest is the package's declaration of what it depends on, whether or
not a build step asks for it.

## Acceptance criteria

1. The fixture's manifest yields `shop` and its seven dependencies;
   `utils` is `libs/utils` and the example's `.path = "../.."` is
   `build.zig.zon`.
