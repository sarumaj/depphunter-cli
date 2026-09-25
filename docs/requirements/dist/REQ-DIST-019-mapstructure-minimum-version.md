---
id: REQ-DIST-019
uuid: e269d2f8-76a9-4583-bbae-1afb9e6e8bd3
title: mapstructure at least v2.4.0
scope: dist
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The build **shall** use `github.com/go-viper/mapstructure/v2` at version v2.4.0
or newer.

## Rationale

Older versions are affected by GO-2025-3787 and GO-2025-3900.

## Acceptance criteria

1. `go.mod` requires `github.com/go-viper/mapstructure/v2` v2.4.0 or newer
   (currently v2.5.0).
2. govulncheck reports neither advisory.

## Notes

The requirement is met by `go.mod`, which carries no annotation.
