---
id: REQ-SUP-021
uuid: 13b21353-eb44-4e68-9776-c34567b45f41
title: Go dependencies from the module proxy
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Go module's requirements from the `go.mod` the
module proxy serves for its version, without fetching an archive, and **shall**
take only its direct (not `// indirect`) requirements.

## Rationale

A module proxy serves a module's own go.mod on its own.

## Acceptance criteria

1. Against a stub proxy the client requests `<module>/@v/<version>.mod` and
   returns its requirements with their versions, pinned.
