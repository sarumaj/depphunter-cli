---
id: REQ-HAXE-007
title: Modules to libraries
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module no file of the repository declares **shall** go, in order: to the
library installed for the file (REQ-HAXE-008) whose class path has it; to
the declared library its leading package segments spell (`tink.core` is
`tink_core`, `thx.promise` `thx.promise`), when that is longer than the
curated table's match; to the standard library - the packages `haxe`,
`sys`, `js`, `flash`, `cpp`, `cs`, `java`, `jvm`, `python`, `lua`, `php`,
`hl`, `neko`, `eval` and the top-level types (`Std`, `Math`,
`StringTools`, `Lambda`, ...), named by their first two package segments
(`haxe.ds`) or first segment (`haxe`, `StringTools`); to the library a
curated table names (`openfl`, `lime`, `flixel`, `hxd`/`h2d`/`h3d`/`hxsl`
heaps, `format`, `utest`, `haxe.ui` haxeui-core, `js.node` hxnodejs,
`thx` thx.core, `cdb` castle, `tink.<x>` `tink_<x>`, ...), declared or
installed, else unresolved; to the declared library spelled. A module of a
package the repository declares, a top-level module, and a qualified name
in code that names no known library **shall** be dropped; any other module
**shall** be an unresolved library named by its first segment.

## Rationale

haxelib libraries do not say which packages they provide, and most do not
spell their names in them (`hxd.Res` is heaps); what is installed is
authoritative, and the table and the declared names cover the rest.

## Acceptance criteria

1. `format.png.Reader` goes to the installed `format`, `gfx.Canvas` to the
   installed `pixels`, `thx.Arrays` to the declared `thx.core`, `js.node.Fs`
   to `hxnodejs`, `haxe.ui.Toolkit` to an unresolved `haxeui-core`,
   `mystery.Thing` to an unresolved `mystery`, `shop.Gone` is dropped, and
   `flixel.FlxG` in code, not declared for the file, is dropped.
