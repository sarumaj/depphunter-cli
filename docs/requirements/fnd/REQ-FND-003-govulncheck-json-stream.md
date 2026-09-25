---
id: REQ-FND-003
uuid: 43bf59fb-b66f-4239-aca5-2c703ad8df7a
title: govulncheck JSON stream
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the JSON stream of `govulncheck -json`: its advisories
and the findings that reach them, keeping for a reachable finding the call site
in the main module and the function from which it is reached.

## Rationale

govulncheck is the one scanner that proves the vulnerable code can be called.

## Acceptance criteria

1. A govulncheck report yields findings whose path and line are the main-module
   call site and whose `reached` names the calling function.
