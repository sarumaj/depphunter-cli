---
id: REQ-DIST-006
uuid: dddd8809-ba7c-44f2-84b1-35d287a01d4e
title: License notices in the release archives
scope: dist
type: constraint
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md §6
  - docs/REQUIREMENTS.md M7
verification:
  - inspection
---

## Statement

The license notices of the vendored and linked libraries **shall** ship with the
source and with every release archive.

## Rationale

Permissive licenses require their notices to accompany redistributions.

## Acceptance criteria

1. Every archive contains `licenses/` with each `web/static/vendor/*.LICENSE`
   file.
2. The source tree holds the same files.

## Notes

Partial: the archives carry the vendored web libraries' licenses, but not the
notices of the linked Go modules (cobra, viper, pflag, tree-sitter runtime and
others), which §6 also names.
