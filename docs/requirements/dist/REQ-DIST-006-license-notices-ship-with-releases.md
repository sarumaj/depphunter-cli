---
id: REQ-DIST-006
uuid: dddd8809-ba7c-44f2-84b1-35d287a01d4e
title: License notices in the release archives
scope: dist
type: constraint
priority: must
status: implemented
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

The archives also carry every `LICENSE*`, `COPYING*` and `NOTICE*` file of the
linked Go modules under `licenses/go/<module path>/`, copied from `vendor/`,
which holds the same files in the source tree.
