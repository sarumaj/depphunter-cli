---
id: REQ-NIX-011
uuid: d60ce800-b093-4b2d-bd40-47b7d5f02f4f
title: Read by a scanner, not the grammar
scope: nix
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Nix **shall** be read by a lexer and parser of its own, not a tree-sitter
grammar: strings with `${...}` interpolation (nested), indented strings with
their `''$`, `'''` and `''\` escapes, `$${` literals, paths (`./x`, `a/b`,
`~/x`, `./x/${y}.nix`), search paths (`<nixpkgs>`), unquoted URIs, `#` and
`/* */` comments; attribute sets, `rec`, `let ... in`, `with`, `assert`,
`inherit`, lambdas with formals and `@` patterns, applications, lists,
selections with `or`, operators. Brackets are matched once; nesting,
recovery and the walk are bounded, so any input **shall** be read in time
linear in its size without a panic.

## Rationale

The vendored grammar was accurate (1 ERROR file of 3922 on home-manager,
disko, devshell, helix, niv and npins) but took 1.9 ms per file and 12.4 s
for nixpkgs' `python-packages.nix` alone; nixpkgs has about 45,000 `.nix`
files. The scanner extracts all of nixpkgs in about 2.5 s of CPU.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
2. Imports inside string text and comments are not read; paths in
   interpolations are.
