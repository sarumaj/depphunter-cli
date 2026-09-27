---
id: REQ-SOLIDITY-010
title: Read by a scanner, not the grammar
scope: solidity
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Solidity sources **shall** be read by a scanner of their own, not a
tree-sitter grammar: a lexer (comments, strings with escapes and
`unicode`/`hex` prefixes, identifiers, numbers, punctuation) and a reader
of declarations whose brackets are matched once, bodies skipped whole.
Any input **shall** be read in time linear in its size without a panic,
and so **shall** the manifests.

## Rationale

The vendored Solidity grammar is accurate (no file with errors among 662
files of forge-std, openzeppelin-contracts, solmate and v4-core) but took
6.7 to 28 ms per file (388 ms for forge-std's safeconsole.sol); the plugin
needs only the source unit's level and the members of contracts.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
