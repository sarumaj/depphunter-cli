---
id: REQ-CS-006
uuid: aaf4d369-49b4-4412-bd23-4679dd5be657
title: Interpolation holes of C# strings walked
scope: cs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - unit
---

## Statement

The C# scanner **shall** walk the interpolation holes of `$"…"` strings
(including `$@"…"`, `@$"…"` and raw `$$"""…"""` forms), so that strings,
character literals and braces inside a hole do not end the literal, while `{{`
is read as a literal brace.

## Rationale

An expression such as `$"{(ok ? "}" : "{")}"` contains quotes and braces that
would otherwise end the string early and desynchronise every following
statement.

## Acceptance criteria

1. In a class holding `$"{(ok ? "x;{" : "b")} {{not a hole}} {'{'}"`, a
   verbatim-interpolated string and a raw interpolated string, a method declared
   after them is reported on its correct line.
