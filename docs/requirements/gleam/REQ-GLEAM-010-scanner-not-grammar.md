---
id: REQ-GLEAM-010
uuid: ad765e4c-1df8-4f66-a1ef-fe5bc8fbc24b
title: Read by a scanner, not the grammar
scope: gleam
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Gleam **shall** be read by a lexer of its own, not a tree-sitter grammar:
`//`, `///` and `////` comments, strings with escapes spanning lines, names,
numbers and punctuation; brackets of all kinds are counted together, a
definition is top level when none is open, and a stray closer never takes
the depth below zero, so any input **shall** be read in time linear in its
size without a panic. TOML is read with BurntSushi/toml.

## Rationale

The vendored grammar parsed 207 of 209 files of gleam-lang/stdlib,
gleam-lang/otp, gleam-lang/packages, lustre and wisp cleanly but took 4.9 ms
per file; everything needed is token-level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
