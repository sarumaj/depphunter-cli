---
id: REQ-FSHARP-002
title: Opens, qualified names and script directives read
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The F# plugin **shall** read as imports every `open X.Y` and
`open type X.Y`, every module abbreviation (`module P = Shop.Pricing`),
the first qualified name in code through each capitalized name
(`Cart.add`, `Shop.Domain.Cart.empty`; names of FSharp.Core and .NET
modules such as `List` or `String` alone are not), and the script
directives `#load "x.fsx"`, `#r "nuget: Id, version"`, `#r "x.dll"`
and `#r "Assembly.Name"`, with the `#I` directories before them. Each
open and name **shall** carry the names it may be relative to: the
enclosing namespaces and modules and their parents, the names opened
before it and those opened names qualified by the enclosing namespaces,
and a script's `#r "nuget: ..."` packages. Nothing inside comments,
strings or char literals **shall** be read.

## Rationale

F# resolves a partial name against its enclosing namespaces and what is
opened, and the files of one namespace use each other's modules without an
`open` - qualified names are most of a project's file edges.

## Acceptance criteria

1. Opens inside `(* *)` (nested, holding `"*)"`), `//`, `"..."`,
   `@"..."`, `"""..."""`, `$"..."` holes and `$$"""..."""` are not
   read; the ones after them are, and `'"'`, `'\''`, `'T` and `i'` do
   not derail the lexer.
2. `build.fsx`'s `#load "helpers.fsx"` with `#I "scripts"` is an import of
   `scripts/helpers.fsx`, and `Helpers.greet` an import of the same file.
