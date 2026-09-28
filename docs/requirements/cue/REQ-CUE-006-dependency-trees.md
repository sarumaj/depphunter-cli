---
id: REQ-CUE-006
title: cue.mod dependency trees
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import found in the governing module's `cue.mod/gen` or
`cue.mod/usr` **shall** resolve to the Go module the nearest `go.mod`
requires for it (the `go` island, pinned as Go pins), the Go module's own
package directory, or the Go standard library (`go-std`); another
generated path **shall** be a `cue` package named by its path. An import
found in `cue.mod/pkg` **shall** be a `cue` package named by the module
file inside it, else by its path.

## Rationale

`cue get go` generates definitions from Go packages; linking them to the
Go module merges them with the Go plugin's node.

## Acceptance criteria

1. `k8s.io/api/core/v1` (gen) and `k8s.io/api/apps/v1` (usr) are
   `k8s.io/api` v0.29.0, `net/http` is Go's standard library,
   `example.com/shop/api` is the `api` directory, and the vendored
   packages are named `github.com/legacy/lib` and `example.org/old`.
