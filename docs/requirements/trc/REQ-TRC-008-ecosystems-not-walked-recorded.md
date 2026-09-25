---
id: REQ-TRC-008
uuid: 6c8f7991-16eb-4962-8862-0db4c9f399b9
title: Ecosystems not walked are recorded
scope: trc
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

When `--resolve-depth` is not 0 and a plugin can neither answer from lock files
nor ask an index, the report **shall** record that plugin as not walked, with
the reason.

## Rationale

Silence is not "no dependencies"; the report is where the difference is kept.

## Acceptance criteria

1. An offline run with `--resolve-depth 1` names the ecosystems that cannot be
   walked offline.
