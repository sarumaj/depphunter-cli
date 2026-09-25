---
id: REQ-GO-002
uuid: 94c61ff0-fd5d-4326-934f-fb1e27e8a4e7
title: Go symbol extraction
scope: go
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Go plugin **shall** extract the top-level declarations of a file as symbols:
functions (`func`), methods (`method`, named `<Receiver>.<method>` with pointers
and type parameters stripped from the receiver), types (`type`), constants
(`const`) and variables (`var`).

## Rationale

These are the units a Go reader navigates by.

## Acceptance criteria

1. `type Server`, `func (s *Server) Start()`, `func main()` and `const Version`
   yield `Server` (type), `Server.Start` (method), `main` (func) and `Version`
   (const).
2. A second `func init()` yields a distinct symbol `init@<line>`.
