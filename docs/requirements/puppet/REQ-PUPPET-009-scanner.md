---
id: REQ-PUPPET-009
title: Read by a lexer, not the grammar
scope: puppet
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Puppet manifests **shall** be read by a lexer of their own (comments,
single- and double-quoted strings with interpolation, heredocs, regular
expressions told from division), not a tree-sitter grammar, and any input
**shall** be read in time linear in its size without a panic.

## Rationale

The vendored tree-sitter Puppet grammar left errors in 104 of 154 files of
puppetlabs-apache and control-repo, at 9 ms per file (the slowest
0.2 s); the lexer takes about 0.1 ms per file.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
