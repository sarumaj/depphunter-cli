---
id: REQ-TOOL-023
uuid: af6d0135-6256-45f9-a5db-f57e6a4994f4
title: Flight is carried by the jet backpack
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M20
verification:
  - ui
  - e2e
---

## Statement

The walker **shall** fly only while the jet backpack is in hand and working;
there **shall** be no key that toggles flight, and putting the jet backpack away
**shall** end the flight.

## Rationale

Flight is a thing carried rather than a mode.

## Acceptance criteria

1. The jet backpack is the only way to fly (M20 acceptance).
2. Stowing the jet backpack in the air makes the walker fall.

## Notes

This replaced the `F` flight toggle of M7 (walk scope).
