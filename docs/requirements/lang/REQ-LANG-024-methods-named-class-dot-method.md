---
id: REQ-LANG-024
uuid: 054d6dc1-ec80-4e40-919a-0449644d2b1c
title: Methods named Class.method
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A method **shall** be extracted as a symbol of kind `method` named
`<Class>.<method>`, where `<Class>` is the enclosing class (or, in Go, the
receiver type).

## Rationale

Method names alone collide across classes; the qualified name stays unique
within the file and reads as it is written in code.

## Acceptance criteria

1. A Go method `func (s *Server) Start()` yields `Server.Start`.
2. A TypeScript method `render` of class `App` yields `App.render`.
3. A Python method `start` of class `Service` yields `Service.start`.
