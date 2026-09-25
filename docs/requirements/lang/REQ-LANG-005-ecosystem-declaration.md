---
id: REQ-LANG-005
uuid: 1c16edb2-f3ed-43e6-83bf-e5e753e14a52
title: Ecosystem declaration
scope: lang
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §4
verification:
  - unit
  - inspection
---

## Statement

A plugin **shall** declare the ecosystems it can emit, each with an identifier,
a display name and whether it is a standard library.

## Rationale

The display name labels the island; the standard-library flag lets the UI hide
those islands by default.

## Acceptance criteria

1. The Go plugin declares `go` ("Go modules") and `go-std` ("Go standard
   library", standard library).
2. An ecosystem node in the graph carries the declared display name.
