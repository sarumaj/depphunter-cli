---
id: REQ-NIM-005
title: Package files
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `.nimble` file **shall** be read as NimScript without running it: its
`packageName`, `version`, `srcDir` and `bin`, and every requirement of its
`requires` and `taskRequires` statements wherever they are (top level,
`when` branches, `feature` and `dev` blocks, task bodies; several strings,
parentheses, `@[...]`, a string listing several separated by commas, a
list continued on the next lines), plus nimble's plain `requires` file
beside it. A requirement is parsed as nimble does: a name and a version
range after the first blank, else a name and a `#ref`, else a name;
features in brackets are dropped; `gh:`, `gl:`, `srht:` and `cb:` forge
aliases are expanded. Each requirement **shall** be an import of its
package: named by its name, or, for a URL, by its repository
(`github.com/x/y`) like other git sources; `nim` itself is the compiler,
not a package, and is dropped; a `file://` requirement in the repository is
an edge to its directory. Each `bin` is an edge to its main module.

`nimble.develop` beside a `.nimble` file, and the develop files it
`includes`, **shall** be read (JSON; paths relative to the file's
directory): a package it lists by a directory of the repository holding a
`.nimble` file is developed in place, so the project's requirement of that
package and the imports of its modules resolve to that directory and its
files, before what nimble installed. A path outside the repository names
nothing, and a garbage or missing file changes nothing.

## Rationale

Requirements are what the package manager installs; a package no module
imports is on the map this way.

## Acceptance criteria

1. The fixture's `shop.nimble` requirements and bin resolve as listed in
   the test; `requires "a", "b"`, `requires("c#head")`, `requires @["d"]`,
   a continued list, an inline `when` and `taskRequires "bench", "i"` are
   read, and `echo "requires m"` is not.
2. With `nimble.develop` developing `../foo` and, through an included
   develop file, `bar`, `requires "foo >= 1.0"` and the URL requirement of
   `nim-bar` are edges to `foo` and `bar`, and `import foo, foo/util, bar`
   resolves to their files; a path outside the repository leaves `baz` a
   package, and a garbage develop file leaves all three packages.
