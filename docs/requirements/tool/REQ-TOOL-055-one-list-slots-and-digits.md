---
id: REQ-TOOL-055
uuid: 4e826779-e6e7-40f0-baae-535b241fc9b6
title: One list for HUD slots and digits
scope: tool
type: constraint
priority: must
status: implemented
verification:
  - ui
  - inspection
---

## Statement

A single ordered list **shall** decide both the HUD's slot order and the digit
keys.

## Rationale

Two lists is how they came apart in the first place.

## Acceptance criteria

1. Every digit selects the tool whose slot shows it.
2. Changing the layout renumbers the keys with it.
