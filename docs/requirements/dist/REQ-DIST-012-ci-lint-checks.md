---
id: REQ-DIST-012
uuid: 154a710d-d707-4081-aefe-43f5a57204d3
title: CI lint checks
scope: dist
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

CI **shall** fail when a Go file (outside `testdata` and `vendor`) is not
gofmt-formatted, when `go mod tidy` changes `go.mod` or `go.sum`, when `go vet`
or staticcheck report a problem, or when markdownlint reports a problem in a
Markdown file.

## Rationale

Uniform formatting and static checks catch mistakes before review.

## Acceptance criteria

1. The `lint` job runs gofmt, `go mod tidy` with a diff check, `go vet`,
   staticcheck and markdownlint.
2. staticcheck is built with the Go version of `go.mod`.

## Notes

staticcheck moved to 2026.2.1, which must be built with a Go at least as new as
the code's. The job also checks the syntax of the browser modules.
