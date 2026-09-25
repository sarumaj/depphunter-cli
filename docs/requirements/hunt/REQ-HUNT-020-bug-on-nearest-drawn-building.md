---
id: REQ-HUNT-020
uuid: 7ca20f05-4a82-4b10-926e-6e6156879e5f
title: Bugs of hidden buildings on nearest drawn one
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A finding whose building is not drawn, such as a file in a collapsed directory,
**shall** place its bug on the nearest ancestor that is drawn.

## Rationale

Collapsing a directory must not hide its findings. The isometric pins use the
same rule, so a bug and its pin agree.

## Acceptance criteria

1. Collapsing a directory moves its files' bugs onto the directory's block.
