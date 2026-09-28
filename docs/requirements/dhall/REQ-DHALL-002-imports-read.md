---
id: REQ-DHALL-002
title: Imports read
scope: dhall
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every import of a Dhall file **shall** be read: local paths (`./x.dhall`,
`../x`, `/abs`, `~/x`), URLs (`https://...`) with their `sha256:` hash
and `using` headers, and `env:VAR`. Each branch of an alternative
(`a ? b ? c`) **shall** count, and an import `as Text`, `as Location` or
`as Bytes` **shall** still be an import. Paths in comments and strings
**shall** not be read.

## Rationale

Dhall's imports are expressions of their own; an alternative is a fallback
the interpreter may take, so each branch is a dependency.

## Acceptance criteria

1. The fixture's `config/app.dhall` yields every import form, the headers
   file of a `using` clause, and nothing from its block comment, its text
   literal or its multi-line string.
