---
id: REQ-HAXE-011
title: Read without the compiler
scope: haxe
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Haxe **shall** be read without running the compiler, haxelib or lix: every
`#if` branch counts, macros are not run (types they generate are unknown),
a qualified name is linked only to a module file (a type declared in another
module of its package, `flixel.group.FlxTypedGroup`, is dropped), types of
the importing module's own package used without an import and what
`import.hx` imports for its directory are not linked, a module of a library
that is not installed is attributed by the declared names and a curated
table, Lime project conditions are not evaluated, and lib.haxe.org is not
asked (no `--online`: it offers Haxe remoting, not a JSON API).

## Rationale

Which branch, macro and library version a build uses is known only to the
build.

## Acceptance criteria

1. Both branches of `#if js ... #elseif ... #else` are read: `js.Browser`,
   `cpp.Lib` and `sys.FileSystem` are imports.
