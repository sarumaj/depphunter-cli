---
id: REQ-HAXE-009
title: Islands
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** declare two islands: `haxelib` ("haxelib libraries";
`haxelib:` is a private-pattern prefix) and `haxe-std` ("Haxe standard
library", hidden like other standard libraries), and emit no other.

## Rationale

Libraries are what a Haxe project depends on; the standard library ships
with the compiler.

## Acceptance criteria

1. Every fixture import resolves to a file, `haxelib`, `haxe-std` or
   nothing.
