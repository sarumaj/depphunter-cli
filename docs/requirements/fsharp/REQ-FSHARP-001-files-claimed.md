---
id: REQ-FSHARP-001
title: F# files claimed
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The F# plugin **shall** claim F# sources (`.fs`, `.fsi`), scripts
(`.fsx`, `.fsscript`), project files (`.fsproj`) and Paket's
`paket.dependencies`, `paket.lock` and `paket.references`, telling the
project and Paket files apart from other files sharing their extensions by
their names. A `.fs` file **shall** be claimed only when the scanner
labeled it F#: the scanner **shall** label a `.fs` file GLSL when its
first 8000 bytes hold a `#version`, `#extension`, `#define`,
`#include` or `#pragma` line, a `precision`, `uniform`, `varying`,
`attribute` or `layout` declaration, `void main` or a `gl_FragColor`
or `gl_FragCoord`, and Forth when a line starts with a `\` comment or is
a colon definition (`: name ... ;`). Nothing under `paket-files/`,
`.paket/` or FAKE's `.fake/`, nor under a `packages/` directory beside
a `paket.dependencies`, **shall** be claimed.

## Rationale

GLSL fragment shaders and Forth use `.fs` too; F# writes none of their
markers (its directives are `#if`, `#else`, `#endif`, `#nowarn`,
`#light`, `#line`, `#load`, `#r` and `#I`, and `void` is reserved).
Paket downloads remote files into `paket-files/` and installs packages into
`packages/`: that is someone else's code.

## Acceptance criteria

1. `shaders/blur.fs` (`#version 330`) and `forth/hello.fs` are labeled
   GLSL and Forth and not claimed; F# files with `#if` and a comment
   mentioning `void main()` stay F#.
2. `build.fsx`, `Pricing.fsi`, `paket.lock`, `paket.references` and
   `Shop.Web.fsproj` are claimed; C# files and `Directory.Packages.props`
   are not, nor `paket-files/fsharp/FAKE/src/Globbing.fs` or
   `.fake/build.fsx/intellisense.fsx`.
