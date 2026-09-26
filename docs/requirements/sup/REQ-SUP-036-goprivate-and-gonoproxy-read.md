---
id: REQ-SUP-036
uuid: c3889f97-877f-4398-b03c-7d7d2330f7f9
title: GOPRIVATE and GONOPROXY are read
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** add the patterns of the `GOPRIVATE` and `GONOPROXY`
environment variables, limited to the Go ecosystem, to the declared private
patterns.

## Rationale

A Go project that has configured its machine needs no configuration here.

## Acceptance criteria

1. GOPRIVATE alone is enough to mark a Go repository's internal modules.
2. The values `none` and `*` add nothing.

## Notes

`GONOSUMDB` is read as well.
