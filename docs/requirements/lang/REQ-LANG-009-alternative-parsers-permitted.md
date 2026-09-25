---
id: REQ-LANG-009
uuid: 9469a08c-4720-4c32-856a-6e142e859953
title: Alternative parsers permitted
scope: lang
type: constraint
priority: may
status: implemented
source:
  - docs/REQUIREMENTS.md §4
  - docs/REQUIREMENTS.md M4
verification:
  - inspection
---

## Statement

A plugin **may** use a parser other than tree-sitter where that is more reliable
or faster; the Go plugin **shall** use the standard library's `go/parser`.

## Rationale

`go/parser` is exact for Go and ships with the toolchain. For C# and PowerShell,
measured grammar performance and error recovery were inadequate, and statement
scanners replace them (scopes `cs`, `ps`).

## Acceptance criteria

1. The Go plugin parses with `go/parser` and links no Go tree-sitter grammar.
