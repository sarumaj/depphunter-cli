---
id: REQ-FSHARP-011
uuid: 7e5412bd-1b85-497e-9fb4-afdd796df18c
title: Read by a scanner, not the grammar
scope: fsharp
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

F# **shall** be read by a lexer of its own and an offside-rule scanner, not
a tree-sitter grammar: nested `(* *)` comments lexing the strings inside
them (`(*)` is the multiplication operator), `//` comments, plain,
verbatim (`@"..."`, `""` escapes; `@"""` is verbatim), triple-quoted,
interpolated (`$"..."`, `$@`, `@$`, `$$"""`) and byte strings, char
literals told from type variables (`'a`, `^T`) and primed names (`x'`),
names quoted in double backticks, and preprocessor lines dropped so both branches
of an `#if` are read. Any input **shall** be read in time linear in its
size without a panic. Project files are read with encoding/xml, Paket
files line by line.

## Rationale

The vendored grammar took 55 to 816 ms per file on Argu, Giraffe and
SAFE-template (17.9 s for Giraffe's Routing.fs), parsed 25 of 97 files with
errors, and did not finish dotnet/fsharp's FSharp.Core sources within 15
minutes; everything needed is token-level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
