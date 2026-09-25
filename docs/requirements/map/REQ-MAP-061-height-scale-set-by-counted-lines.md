---
id: REQ-MAP-061
uuid: 96c61dc7-05c2-47fb-8dac-0e57102a8674
title: Height scale set by counted lines only
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M32
verification:
  - ui
---

## Statement

The system **shall** set the height scale's maximum from the largest counted
line count of any file, and **shall** cap a file's drawn height at that maximum,
so that an unread file tops out level with the longest file.

## Rationale

A large blob must not flatten the whole city to make room for itself.

## Acceptance criteria

1. Adding a 4 MB binary to a repository leaves the height of every other
   building unchanged, and the binary is no taller than the longest file.
