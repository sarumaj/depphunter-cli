---
id: REQ-HIST-004
uuid: 12d1f930-19ab-4667-be66-e8bf8136e74f
title: Authors keyed by e-mail, shown by name
scope: hist
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

The history **shall** identify an author by the lower-cased e-mail address (by
the name where no e-mail is recorded) and list each author once, by display
name.

## Rationale

The same person commits under several spellings of the name but usually one
address.

## Acceptance criteria

1. Two commits with the same e-mail in different case and different names count
   as one author, shown by the first name read.
