---
id: REQ-SUP-036
title: GOPRIVATE and GONOPROXY are read
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** add the patterns of `GOPRIVATE`, `GONOPROXY` and
`GONOSUMDB`, limited to the Go ecosystem, to the declared private patterns,
each read as the go command reads it: from the environment when set there,
else from the go env file (`GOENV`, else `go/env` in the user configuration
directory; none for `GOENV=off`).

## Rationale

A Go project that has configured its machine needs no configuration here.

## Acceptance criteria

1. GOPRIVATE alone is enough to mark a Go repository's internal modules.
2. The values `none` and `*` add nothing.
3. GOPRIVATE written by `go env -w` is read from the go env file; the
   environment's value wins over it.

## Notes

`GONOSUMDB` is read as well. There is no `GONOSUMCHECK` variable in the go
command (earlier documentation named one); `GOFLAGS` carries no module
patterns.
