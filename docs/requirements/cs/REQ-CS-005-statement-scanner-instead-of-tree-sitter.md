---
id: REQ-CS-005
uuid: 5a596c01-8d7b-403b-a5f6-a83b62d90bd5
title: C# statement scanner instead of tree-sitter
scope: cs
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The C# plugin **shall** read `using` directives and declarations with a
statement scanner rather than a tree-sitter grammar; the scanner **shall** split
source into statements at `;`, `{` and `}` outside parentheses, and **shall**
skip line and block comments, preprocessor lines, character literals and
regular, verbatim, interpolated and raw string literals, so that their contents
never yield a directive, a declaration or a block boundary.

## Rationale

The pure-Go tree-sitter C# grammar took 24 s for Serilog's 216 files (8 s for
one 59 KB file). `using` directives and declarations are statement-level, so a
scanner that understands comments, strings and braces suffices; Serilog then
takes 20 ms.

## Acceptance criteria

1. `using` directives inside comments, raw strings and interpolated strings are
   not reported.
2. A `{` inside a string, a verbatim string or a character literal does not open
   a block.
3. Classes, interfaces, structs, enums, records, delegates and methods are
   reported as symbols with their lines; a constructor is not a method.
4. Serilog's sources are analyzed in well under a second.

## Notes

Criterion 4 is a measurement from the design log (M4), not an automated test.
