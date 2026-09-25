---
id: REQ-EXT-001
uuid: e82395d9-3ead-417d-83be-67ac37922d02
title: Activity-bar container with three views
scope: ext
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
  - README.md Use
  - README.md The panel beside the code
verification:
  - extension
  - inspection
---

## Statement

The extension **shall** contribute an activity-bar container of its own holding
three views: **Maps** (the folders that can be mapped), **Dependencies** (the
dependency tree) and **Backpack** (the collected findings).

## Rationale

The panel beside the code and the map are one interface; the tree and the
backpack sit beside the list of maps so that the catch and the graph can be
worked through where the fixing happens.

## Acceptance criteria

1. The activity bar shows a depphunter icon whose container lists the views
   Maps, Dependencies and Backpack, in that order.
2. Each of the three views has a registered data provider after activation.
