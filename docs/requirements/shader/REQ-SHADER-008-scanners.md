---
id: REQ-SHADER-008
title: Read by scanners, not the grammars
scope: shader
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

GLSL, HLSL and WGSL **shall** be read by scanners of their own (a C-like
token scanner over the C/C++ plugin's directive scan, and a WGSL lexer),
not tree-sitter grammars, and any input **shall** be read in time linear in
its size without a panic.

## Rationale

Measured on the smoke repositories, the vendored grammars left errors in
31 of 345 GLSL files (1.9 ms per file), 154 of 377 HLSL files (9.3 ms, up
to 177 ms), and 114 of 117 WGSL and 195 of 211 WESL files (3.6 to 4.1 ms:
naga_oil's directives and WESL's imports are no WGSL); the CUDA grammar
took 41 ms per CUDA file, up to 1.5 s, with 42 error files of 221. The
scanners take well under 0.2 ms per file.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read in each dialect without a panic within the time bound.
