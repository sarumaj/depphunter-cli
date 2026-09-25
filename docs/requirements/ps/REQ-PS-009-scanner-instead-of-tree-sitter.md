---
id: REQ-PS-009
uuid: 434f1a25-a4af-41cc-8f54-7a7ed8827471
title: PowerShell scanner instead of tree-sitter
scope: ps
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** read dependencies and definitions with a
statement scanner rather than a tree-sitter grammar; the scanner **shall** split
source into statements on newlines, `;`, `{` and `}`, **shall** skip line
comments, `<# … #>` block comments, string contents and here-string contents,
**shall** join lines continued with a backtick, a trailing pipe or comma, or an
open parenthesis, and **shall not** split inside a `@{ … }` hashtable.

## Rationale

The pure-Go tree-sitter PowerShell grammar turns `Import-Module -Name A, B` into
an error node that swallows the following lines. Dependencies and declarations
are statement-level, so a scanner suffices.

## Acceptance criteria

1. `Import-Module` inside a here-string or a block comment is not reported.
2. `Write-Host 'not # a comment'; Import-Module A # trailing comment` yields
   `A`.
3. Functions, filters, workflows, classes and class methods are reported as
   symbols with their lines.
4. Two statements on one line separated by `;` are both read, with that line.
