---
id: REQ-HAXE-009
uuid: 923d1c94-1394-42ae-8c9a-8ddbc67a58ba
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
