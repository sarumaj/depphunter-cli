---
id: REQ-DHALL-007
title: Read by the Dhall reader, not the grammar
scope: dhall
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Dhall **shall** be read by the lexer and reader in `internal/lang/dhall`
(shared with the PureScript plugin), not the vendored tree-sitter grammar,
and any input **shall** be read in time linear in its size without a
panic.

## Rationale

The vendored grammar parsed 2,158 files of the Prelude and
dhall-kubernetes without an error but took 2.5 ms per file (the slowest
0.4 s); the reader takes about 0.02 ms per file, which matters for
dhall-kubernetes' 45,000 generated files.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
