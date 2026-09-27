---
id: REQ-PROTO-005
title: Imports from Buf Schema Registry dependencies
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import that no import root holds **shall** resolve to a Buf Schema Registry
module (the `buf` island, named `host/owner/repository`, lower-cased) that the
importer's Buf configuration declares in `deps` - its v2 workspace's and its v1
module's `buf.yaml` - or that the `buf.lock` beside that configuration records:
the module REQ-PROTO-006's table names for the path when one of its candidates
is declared, else the declared module whose repository, or alone among them
whose owner, is the path's first directory (compared without case, `-`, `_` or
`.`). A file no Buf configuration governs **shall** see the modules of every
`buf.yaml` in the repository, shallowest first.

## Rationale

`buf build` finds an import in the workspace or in one of the modules
`buf.yaml` depends on, and the lock names the exact commit it uses, including
modules that are only dependencies of dependencies.

## Acceptance criteria

1. With `buf.build/googleapis/googleapis` declared and locked,
   `import "google/api/annotations.proto"` resolves to it, pinned by the lock's
   commit.
2. `import "payments/v1/pay.proto"` resolves to the declared
   `buf.build/acme/payments`; in a v2 workspace whose lock alone records
   googleapis, `import "google/type/money.proto"` resolves to it.
3. A module declared by another workspace's `buf.yaml` does not resolve a file
   governed by a configuration that does not declare it.
