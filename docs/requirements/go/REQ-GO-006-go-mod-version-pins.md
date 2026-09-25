---
id: REQ-GO-006
uuid: 5f8a92bb-63f7-4eba-a129-a1711a32bc81
title: go.mod version pins
scope: go
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The Go plugin **shall** treat the version a `go.mod` requires as pinned, since
it is the version the build selects.

## Rationale

Go's minimal version selection builds exactly the versions `go.mod` states;
unlike a `Cargo.toml` entry, a `go.mod` version is not a range.

## Acceptance criteria

1. A module required at `v1.8.0` is not floating.
2. A module required at a pseudo-version is not floating.

## Notes

The general floating rule is scope `sup`.
