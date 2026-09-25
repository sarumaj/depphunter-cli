---
id: REQ-MOD-013
uuid: 5c9c5140-7e19-4168-9579-7d95519ab671
title: Generated TypeScript declaration
scope: mod
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M17
verification:
  - unit
---

## Statement

The TypeScript declaration of the graph document used by the VS Code extension
(`extension/src/graph.ts`) **shall** be generated from the Go declaration in
`internal/graph/graph.go`, and the ordinary test run **shall** fail, naming the
generated file, when the committed declaration differs from what the Go
declaration generates.

## Rationale

A hand-maintained copy lets a field be added on one side and silently not read
on the other, which is not a compile error anywhere.

## Acceptance criteria

1. `go test ./internal/graph -update` rewrites `extension/src/graph.ts`.
2. Adding a field to `graph.Node` without regenerating makes `go test
   ./internal/graph` fail with a message naming `extension/src/graph.ts`.
3. Doc comments of the Go types and fields appear in the generated file.
