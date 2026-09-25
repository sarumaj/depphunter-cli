---
id: REQ-EXT-028
uuid: febe5dfd-cca6-47f5-bf95-a8415511f209
title: Maps view lists folders and servers
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Use
verification:
  - extension
---

## Statement

The Maps view **shall** list the window's folders and any other mapped folder,
show which have a running server and its address without the session token, open
the map when an entry is selected, and offer restart and stop on a running
entry.

## Rationale

The view is the entry point from the activity bar; the token is a secret and not
for the screen.

## Acceptance criteria

1. A folder with a running server has context `running` and describes its
   address.
2. Neither description nor tooltip contains the token.
