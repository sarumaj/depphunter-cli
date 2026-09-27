---
id: REQ-HAXE-001
uuid: 6e399561-9fa6-455a-9b44-fdea637789cb
title: Files claimed
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Haxe plugin **shall** claim Haxe modules (`.hx`), the compiler's build
files (`.hxml`), haxelib's `haxelib.json`, lix's pins
(`haxe_libraries/<name>.hxml`, told apart from build files by their
directory) and Lime/OpenFL project files named `Project.xml`,
`project.xml` or `include.xml`. Since `project.xml` is a name other tools
use too, such a file **shall** be claimed only when its root element is
`<project>` (`<extension>` for a library's `include.xml`) and it names a
`<haxelib>`, a `<source>`, a `<classpath>` or an `<include haxelib>`.
Nothing in a local haxelib repository (`.haxelib/`) **shall** be claimed.

## Rationale

A Haxe build is described by `.hxml` files, a library by `haxelib.json`, a
lix project by its `haxe_libraries/` and an OpenFL game by `Project.xml`;
`project.xml` is also Ant's, IntelliJ's and other tools' name, and
`.haxelib/` holds copies of other libraries' sources.

## Acceptance criteria

1. The fixture's `.hx` files, `build.hxml`, `common.hxml`,
   `game/build.hxml`, the five `game/haxe_libraries/*.hxml` (class `lix`),
   `haxelib.json` and `app/Project.xml` are claimed; `docs/project.xml` (an
   Ant file), `app/shared.xml` and everything under `.haxelib/` are not.
2. An Ant `<project>`, a Maven POM, an MSBuild `<Project>` and a commented
   out `<project>` are not Lime project files.
