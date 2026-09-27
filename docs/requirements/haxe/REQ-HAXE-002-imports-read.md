---
id: REQ-HAXE-002
title: Imports read
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a module's `import` and `using` declarations,
including sub-type imports (`import a.b.C.D`), static field imports
(`import a.b.C.field`), wildcards (`import a.b.*`, `import a.b.C.*`), aliases
(`as`, `in`) and `std.`-prefixed paths, and the qualified type names its code
uses (`haxe.Json.parse(...)`, `new a.b.C()`, `@:build(a.b.Macro.build())`)
that it does not import. A qualified name of one lower-case segment and a
capitalized constant (`gl.TEXTURE_2D`, `key.ID`) **shall** not be read as a
type. Every branch of `#if`/`#elseif`/`#else` **shall** count, and nothing
in comments, `"..."` and `'...'` strings (including `${...}`
interpolations) or `~/.../` regular expressions **shall** be read.

## Rationale

Haxe names other modules by imports and by fully qualified paths in code;
conditional compilation selects targets that are all part of the project.

## Acceptance criteria

1. `src/shop/Main.hx`'s imports, its `#if js`/`#elseif`/`#else` imports and
   the qualified names `haxe.crypto.Md5` and `flash.display.Sprite` are
   read; `shop.model.Cart` in code is not repeated (it is imported), and
   `key.ID` is not read.
2. Fake imports in line and block comments, double- and single-quoted
   strings, an interpolation containing `"}"` and a nested string, and a
   regular expression are not read.
