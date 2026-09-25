---
id: REQ-CITY-028
uuid: 386a6484-80aa-4d68-b3a6-8e04d89d20b8
title: Walker height follows the bridge arch
scope: city
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

On a bridge the walker's height **shall** follow the arch, and the deck
**shall** carry the walker only while the whole body (radius 0.12) is within the
deck.

## Rationale

Asking at the body's corners let a walker hang over the railing.

## Acceptance criteria

1. Walking across a bridge rises and falls with the arch.
2. A walker cannot be walked out through the railing.
