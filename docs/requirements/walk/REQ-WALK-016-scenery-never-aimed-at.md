---
id: REQ-WALK-016
uuid: 583bc7e1-caba-421d-b81a-f68e577c9749
title: Shore and block underfoot are never aimed at
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The aim **shall** ignore the shore (land boxes) and the terrace the walker is
standing in: neither **shall** be tinted, described by a hover card, tagged or
selected from walk mode.

## Rationale

They fill the lower half of the view; treating them as targets made every
downward glance a hover card.

## Acceptance criteria

1. Looking down at the street shows no hover card and no highlight.
2. Looking at a neighboring building highlights it.
