---
id: REQ-PROTO-003
title: Protocol Buffers declarations as symbols
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols a file's `package` (kind `package`),
its messages (`message`, nested ones named `Outer.Inner`, a proto2 `group`
as the nested message it declares), enums (`enum`), services (`service`), the
rpc methods of a service (`rpc`, named `Service.Method`), `extend` blocks
(`extend`, named by the extended type) and oneofs (`oneof`, named
`Message.oneof`), each at the line of its name.

## Rationale

Messages and services are what other files and other languages' generated code
refer to; their nesting is part of their name.

## Acceptance criteria

1. A message `Invoice` with a nested `Line` holding an enum `Kind` and a oneof
   `payer` gives `Invoice`, `Invoice.Line`, `Invoice.Line.Kind` and
   `Invoice.payer`; a service's rpcs are `InvoiceService.GetInvoice`.
2. `optional group Actor = 1 { ... }` in message `Entry` gives the message
   `Entry.Actor`; a field named `message` or `group` is not a declaration.
