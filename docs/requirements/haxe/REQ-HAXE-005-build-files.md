---
id: REQ-HAXE-005
uuid: bd9fb297-73d1-4ffe-b8a1-51991d8d724e
title: Build files
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `.hxml` files as the compiler splits them (a
line starting with `-` is a flag and its value, any other line one
argument, `#` a comment), over all `--next` and `--each` sections, and make
imports of `-lib`/`-L`/`--library name[:version]` (the library),
`-cp`/`-p`/`--class-path` (the directory), `-main`/`-m`/`--main`/`--run`
and bare module arguments (the module's file under the file's class paths),
`-resource file@name` (the file), a bare `other.hxml` (that file) and the
classes a `--macro` call names; `--cwd` changes the base of later paths and
values with `${...}` or template placeholders (`::OUT::`) are skipped. It
**shall** read `haxelib.json`'s `dependencies` (each an import of the
library) and `classPath` (the directory), and a Lime project file's
`<haxelib name version>` and `<include haxelib>` (libraries),
`<source path>` and `<classpath name|path>` (directories), `<app main>` (the
main class) and `<include path>` (the file, whose declarations count as the
including file's), with a tag scanner that accepts what Lime files hold
beyond XML (`if="${a < b}"`); conditions (`if`, `unless`) are not
evaluated. The libraries a file may import are those its own and its
ancestor directories' build files declare, and those lix pins for it.

## Rationale

Haxe keeps no project model besides these files; the libraries a build
names are the dependencies.

## Acceptance criteria

1. `build.hxml`'s `-cp`, `--class-path`, `-lib`, `--library`, `-main`,
   `-resource`, `--macro`, the root module `shop.util.Money` and
   `common.hxml` resolve as the fixture test lists.
2. `app/Project.xml`'s `<app main="app.Main">` resolves to
   `app/source/app/Main.hx`, `<haxelib name="${custom-backend}">` is
   skipped, the commented out `<haxelib>` is not read, and `actuate`, which
   the included `shared.xml` declares, is declared for `app/`.
